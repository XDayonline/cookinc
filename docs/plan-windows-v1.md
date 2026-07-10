# Cookinc — Windows side implementation plan

## Goal

Implement the Windows daemon side of cookinc: read Chrome cookies from SQLite,
decrypt via DPAPI, filter by allowlist, and push to the Linux sink.

## Files to create

### 1. `internal/chrome/read_windows.go`

Windows-only implementation of the `chrome.Reader` interface.

```go
// Package chrome Windows implementation using DPAPI.
// Build tag: windows

type WindowsReader struct {
    dbPath string
}

// Open Chrome Cookies SQLite (locked by Chrome — must close Chrome first or
// use Volume Shadow Copy or copy the file while Chrome is running).
// SQLite path: %LOCALAPPDATA%\Google\Chrome\User Data\Default\Network\Cookies

// ReadCookies opens the SQLite DB, queries:
//   SELECT host_key, name, value, path, expires_utc, is_secure,
//          is_httponly, priority, samesite, source_port
//   FROM cookies
//   ORDER BY host_key

// For each row, decrypt `value` using DPAPI:
//   - Call CryptUnprotectData from win32/dpapi
//   - The encrypted blob starts with 3 bytes (v10 prefix) + encrypted content
//   - Actually Chrome on Windows 127+ uses App-Bound Encryption (ABE),
//     check the "appbound" flag or use the new Chrome key storage.
//     For Chrome < 127, DPAPI works. For >= 127, use chrome's own
//     encryption key (stored in %LOCALAPPDATA%\...\Chrome\User Data\Local State).

// Filter result against the allowlist (passed in params).
// Return []protocol.Cookie.

// DBPath() returns the configured dbPath.
```

### 2. `internal/chrome/crypto_windows.go`

DPAPI crypto helpers.

```go
// Package crypto Windows implementation using DPAPI.

import "golang.org/x/sys/windows"

// decryptDPAPI(encrypted []byte) ([]byte, error)
//   - Calls win32 CryptUnprotectData
//   - See docs: https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptunprotectdata

// For Chrome >= 127 with App-Bound Encryption:
//   - Read encrypted key from %LOCALAPPDATA%\.config\google-chrome\User Data\Local State
//     (JSON: os_crypt.encrypted_key)
//   - Decrypt with DPAPI
//   - Use that key to AES-256-GCM decrypt cookie values
```

### 3. `internal/chrome/paths_windows.go`

```go
// Package chrome path detection for Windows.

func defaultChromeCookiesPath() string {
    return filepath.Join(os.Getenv("LOCALAPPDATA"),
        "Google", "Chrome", "User Data", "Default", "Network", "Cookies")
}

func defaultChromeLocalStatePath() string {
    return filepath.Join(os.Getenv("LOCALAPPDATA"),
        "Google", "Chrome", "User Data", "Local State")
}
```

### 4. `cmd/cookinc/commands/init.go`

Cobra command `cookinc init`.

```go
// Creates ~\.config\cookinc\source.yaml with:
// - Interactive prompts: sink URL, shared secret, allowlist domains
// - Or flags: --sink-url, --secret, --allowlist

func initCmd() *cobra.Command {
    // ...
}
```

### 5. `cmd/cookinc/commands/start.go`

Cobra command `cookinc start`.

```go
// Starts the file watcher + sync loop:
// 1. Load config from source.yaml
// 2. Set up fsnotify on Chrome Cookies file
// 3. On change: read DB, decrypt DPAPI, filter allowlist, encrypt, POST
// 4. Log errors, keep running
```

## Dependencies needed

```
go get github.com/spf13/cobra
go get github.com/fsnotify/fsnotify
go get github.com/mattn/go-sqlite3          # CGO required
go get golang.org/x/sys/windows              # For DPAPI / win32 APIs
```

## Chrome cookie encryption note

Chrome < 127: `value` column is AES-128-CBC with DPAPI-encrypted key.
Chrome >= 127 (Windows): App-Bound Encryption (ABE). The key is in `Local State`
JSON under `os_crypt.encrypted_key`. Decrypt that with DPAPI first, then use it
for AES-256-GCM decrypt of cookie values.

Reference: [ChromeOSCrypt](https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/)

## Test plan

1. Close Chrome on Windows
2. Run `cookinc.exe init --allowlist github.com`
3. Run `cookinc.exe start` — should read cookies and push to sink URL
4. Verify with `curl http://localhost:9898/get_cookies(github.com)` on the Linux sink

## Reference repos for inspiration

- **agentcookie** (macOS, mais architecture solide) : https://github.com/mvanhorn/agentcookie
  - `internal/chrome/read.go` — pattern lecture SQLite Chrome
  - `internal/chrome/crypto_test.go` — pattern decrypt/encrypt tests
  - `internal/protocol/` — structure enveloppe de sync

- **HackBrowserData** (cross-platform, inclut Windows DPAPI) : https://github.com/moonD4rk/HackBrowserData
  - `pkg/decrypt/decrypt.go` — DPAPI decrypt pattern en Go
  - `pkg/browser/chrome.go` — lecture Chrome DB + Local State
  - Référence pour le ABE (App-Bound Encryption) Chrome >= 127

- **chlonium** (Windows Chrome → re-encrypt) : https://github.com/rxwx/chlonium
  - Décryptage DPAPI + ré-encryptage AES-128-CBC
  - Utile pour comprendre le format v10/v11

## PR structure

One PR on the existing repo with all Windows files + updated `cmd/cookinc/main.go`.
Do NOT remove any existing files.
