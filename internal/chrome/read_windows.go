package chrome

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/XDayonline/cookinc/internal/protocol"
)

// WindowsReader implements the Reader interface for Chrome on Windows,
// using DPAPI for cookie value decryption.
type WindowsReader struct {
	dbPath         string
	localStatePath string
	key            []byte
}

// NewWindowsReader creates a new WindowsReader.
//
// If dbPath or localStatePath are empty, default paths are used.
// The encryption key is loaded and cached from Chrome's Local State.
func NewWindowsReader(dbPath, localStatePath string) (*WindowsReader, error) {
	if dbPath == "" {
		dbPath = defaultChromeCookiesPath()
	}
	if localStatePath == "" {
		localStatePath = defaultChromeLocalStatePath()
	}

	key, err := readEncryptionKey(localStatePath)
	if err != nil {
		return nil, fmt.Errorf("chrome: load encryption key: %w", err)
	}

	return &WindowsReader{
		dbPath:         dbPath,
		localStatePath: localStatePath,
		key:            key,
	}, nil
}

var cdpOnce sync.Once

// ReadCookies reads cookies from Chrome, trying CDP first (for Chrome 127+
// with App-Bound Encryption), falling back to direct SQLite + DPAPI decryption.
func (r *WindowsReader) ReadCookies(allowlist []string) ([]protocol.Cookie, error) {
	if len(allowlist) == 0 {
		return nil, nil
	}

	// Try CDP (works with Chrome 127+ App-Bound Encryption)
	cookies, err := cdpReadCookies(allowlist)
	if err == nil {
		return cookies, nil
	}

	// One-shot attempt to relaunch Chrome with debug port
	cdpOnce.Do(func() {
		if cdpCheckPort("http://localhost:9222") {
			return
		}
		log.Println("chrome: App-Bound Encryption detected (Chrome 127+)")
		log.Println("chrome: to read cookies, close Chrome then start with:")
		log.Println(`chrome:   "C:\Program Files\Google\Chrome\Application\chrome.exe" --remote-debugging-port=9222`)
		log.Println("chrome: or let cookinc try: relaunching Chrome with debug port...")
		if launchErr := LaunchWithDebugPort(); launchErr != nil {
			log.Printf("chrome: relaunch failed (try manually): %v", launchErr)
		}
	})

	// Try CDP again after relaunch
	cookies, err = cdpReadCookies(allowlist)
	if err == nil {
		return cookies, nil
	}

	// Fall back to direct DB decryption (may fail for v20)
	return r.readCookiesDB(allowlist)
}

// readCookiesDB reads cookies from Chrome's SQLite store directly,
// using DPAPI decryption (works for Chrome < 127; partial for 127+).
func (r *WindowsReader) readCookiesDB(allowlist []string) ([]protocol.Cookie, error) {
	tmpPath, err := r.copyDB()
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpPath)

	db, err := sql.Open("sqlite", tmpPath+"?mode=ro&_journal_mode=off")
	if err != nil {
		return nil, fmt.Errorf("chrome: open temp db: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT host_key, name, encrypted_value, path, expires_utc, is_secure, is_httponly
		FROM cookies
		WHERE length(encrypted_value) > 0
		ORDER BY host_key`)
	if err != nil {
		return nil, fmt.Errorf("chrome: query cookies: %w", err)
	}
	defer rows.Close()

	var cookies []protocol.Cookie
	for rows.Next() {
		var c protocol.Cookie
		var valBlob []byte
		err := rows.Scan(&c.HostKey, &c.Name, &valBlob, &c.Path,
			&c.ExpiresUTC, &c.IsSecure, &c.IsHTTPOnly)
		if err != nil {
			log.Printf("chrome: scan row: %v", err)
			continue
		}

		if !domainMatches(c.HostKey, allowlist) {
			continue
		}

		plaintext, err := decryptCookieValue(valBlob, r.key, c.Name)
		if err != nil {
			log.Printf("chrome: decrypt %s/%s: %v", c.HostKey, c.Name, err)
			continue
		}

		c.Value = string(plaintext)
		cookies = append(cookies, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chrome: rows iteration: %w", err)
	}

	return cookies, nil
}

// DBPath returns the path to the Chrome Cookies SQLite file.
func (r *WindowsReader) DBPath() string {
	return r.dbPath
}

// domainMatches defined in match.go
