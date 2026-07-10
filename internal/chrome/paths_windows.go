// Package chrome provides Windows Chrome path detection.
//
// Build constraint: windows only.
package chrome

import (
	"os"
	"path/filepath"
)

// defaultChromeCookiesPath returns the default path to Chrome's Cookies SQLite
// database on Windows.
func defaultChromeCookiesPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"),
		"Google", "Chrome", "User Data", "Default", "Network", "Cookies")
}

// defaultChromeLocalStatePath returns the default path to Chrome's Local State
// JSON file on Windows.
func defaultChromeLocalStatePath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"),
		"Google", "Chrome", "User Data", "Local State")
}
