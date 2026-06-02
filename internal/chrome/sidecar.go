// Package chrome provides Linux-specific cookie storage management.
// For V1, cookinc-mcp stores cookies in a sidecar SQLite (no Chrome
// SQLite dependency) and serves them via MCP. Writing directly to
// Chrome's SQLite will be added in V1.1.
package chrome

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/XDayonline/cookinc/internal/protocol"
)

// Sidecar manages the local cookie sidecar SQLite database.
type Sidecar struct {
	dbPath string
	db     *sql.DB
}

// NewSidecar opens or creates the sidecar database.
// The DB path defaults to ~/.cookinc/cookies.db.
func NewSidecar(dbPath string) (*Sidecar, error) {
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("chrome: home dir: %w", err)
		}
		dbPath = filepath.Join(home, ".cookinc", "cookies.db")
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("chrome: mkdir %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("chrome: open sidecar: %w", err)
	}

	s := &Sidecar{dbPath: dbPath, db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("chrome: migrate: %w", err)
	}

	return s, nil
}

// DBPath returns the sidecar database path.
func (s *Sidecar) DBPath() string {
	return s.dbPath
}

// Close closes the database.
func (s *Sidecar) Close() error {
	return s.db.Close()
}

// WriteCookies replaces all cookies in the sidecar with the provided set.
// This is simpler than upsert logic and fine for the sidecar use case.
func (s *Sidecar) WriteCookies(cookies []protocol.Cookie) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("chrome: begin tx: %w", err)
	}
	defer tx.Rollback()

	// Clear existing cookies
	if _, err := tx.Exec("DELETE FROM cookies"); err != nil {
		return fmt.Errorf("chrome: clear: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO cookies (host_key, name, value, path, expires_utc, is_secure, is_httponly, priority, samesite, source_port, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("chrome: prepare: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, c := range cookies {
		_, err := stmt.Exec(c.HostKey, c.Name, c.Value, c.Path, c.ExpiresUTC, c.IsSecure, c.IsHTTPOnly, c.Priority, c.SameSite, c.SourcePort, now)
		if err != nil {
			return fmt.Errorf("chrome: insert %s/%s: %w", c.HostKey, c.Name, err)
		}
	}

	return tx.Commit()
}

// GetCookies returns all cookies matching the given domain.
// Domain matching is flexible: "github.com" matches ".github.com", "www.github.com", etc.
func (s *Sidecar) GetCookies(domain string) ([]protocol.Cookie, error) {
	rows, err := s.db.Query(`
		SELECT host_key, name, value, path, expires_utc, is_secure, is_httponly, priority, samesite, source_port
		FROM cookies
		WHERE host_key = ? OR host_key = ? OR host_key LIKE ?
		ORDER BY host_key, name`, domain, "."+domain, "%."+domain)
	if err != nil {
		return nil, fmt.Errorf("chrome: query: %w", err)
	}
	defer rows.Close()

	var cookies []protocol.Cookie
	for rows.Next() {
		var c protocol.Cookie
		if err := rows.Scan(&c.HostKey, &c.Name, &c.Value, &c.Path,
			&c.ExpiresUTC, &c.IsSecure, &c.IsHTTPOnly,
			&c.Priority, &c.SameSite, &c.SourcePort); err != nil {
			return nil, fmt.Errorf("chrome: scan: %w", err)
		}
		cookies = append(cookies, c)
	}
	return cookies, nil
}

// ListDomains returns all unique host_keys in the sidecar.
func (s *Sidecar) ListDomains() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT host_key FROM cookies ORDER BY host_key")
	if err != nil {
		return nil, fmt.Errorf("chrome: list domains: %w", err)
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		domains = append(domains, d)
	}
	return domains, nil
}

// Count returns the total number of cookies stored.
func (s *Sidecar) Count() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM cookies").Scan(&count)
	return count, err
}

// LastUpdated returns the timestamp of the most recent update, or zero time if empty.
func (s *Sidecar) LastUpdated() (time.Time, error) {
	var ts int64
	err := s.db.QueryRow("SELECT COALESCE(MAX(updated_at), 0) FROM cookies").Scan(&ts)
	if err != nil {
		return time.Time{}, err
	}
	if ts == 0 {
		return time.Time{}, nil
	}
	return time.Unix(ts, 0), nil
}

func (s *Sidecar) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS cookies (
			host_key    TEXT NOT NULL,
			name        TEXT NOT NULL,
			value       TEXT NOT NULL,
			path        TEXT NOT NULL DEFAULT '/',
			expires_utc INTEGER NOT NULL DEFAULT 0,
			is_secure   INTEGER NOT NULL DEFAULT 0,
			is_httponly INTEGER NOT NULL DEFAULT 0,
			priority    INTEGER NOT NULL DEFAULT 0,
			samesite    INTEGER NOT NULL DEFAULT 0,
			source_port INTEGER NOT NULL DEFAULT 0,
			updated_at  INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (host_key, name, path)
		);

		CREATE INDEX IF NOT EXISTS idx_cookies_host ON cookies(host_key);`)
	return err
}
