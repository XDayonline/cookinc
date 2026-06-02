package chrome

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

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

// ReadCookies reads cookies from Chrome's SQLite store, decrypts them,
// and filters by the given allowlist.
//
// The database is copied to a temp file (with retry) to avoid locking
// issues with a running Chrome instance.
func (r *WindowsReader) ReadCookies(allowlist []string) ([]protocol.Cookie, error) {
	if len(allowlist) == 0 {
		return nil, nil
	}

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
		SELECT host_key, name, value, path, expires_utc, is_secure, is_httponly
		FROM cookies
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

		plaintext, err := decryptCookieValue(valBlob, r.key)
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

// domainMatches checks if cookieDomain matches any domain in the allowlist.
//
// "github.com" matches cookies for "github.com", ".github.com",
// "www.github.com", "api.github.com", etc.
func domainMatches(cookieDomain string, allowlist []string) bool {
	cd := strings.ToLower(cookieDomain)
	for _, allowed := range allowlist {
		a := strings.ToLower(allowed)
		if cd == a || cd == "."+a || strings.HasSuffix(cd, "."+a) {
			return true
		}
	}
	return false
}
