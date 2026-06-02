// Package chrome handles reading and writing Chrome's cookie store.
// Windows reads use DPAPI for decryption; Linux writes re-encrypt
// with the local Chrome Safe Storage key.
package chrome

import (
	"github.com/XDayonline/cookinc/internal/protocol"
)

// Reader reads and decrypts cookies from Chrome's SQLite store.
// Platform-specific implementations:
//   - Windows: DPAPI decrypt
//   - macOS:   Keychain decrypt (future)
type Reader interface {
	// ReadCookies returns all cookies for the configured Chrome profile,
	// decrypted and filtered by the provided domain allowlist.
	ReadCookies(allowlist []string) ([]protocol.Cookie, error)

	// DBPath returns the path to the Chrome Cookies SQLite file being read.
	DBPath() string
}

// Writer writes cookies into Chrome's SQLite store, re-encrypting
// them for the local machine.
// Platform-specific:
//   - Linux: AES-128-CBC with key from Local State
//   - macOS: Keychain encrypt (future)
type Writer interface {
	// WriteCookies inserts or updates cookies in the local Chrome store.
	WriteCookies(cookies []protocol.Cookie) error

	// DBPath returns the path to the Chrome Cookies SQLite file being written.
	DBPath() string
}
