# AGENTS.md

> **Open-source project.** No confidential data, credentials, secrets, cookies, API keys, tokens, or personal information is ever stored in this repository.
> 
> **All secrets and cookies are local-only** — stored in `~/.config/cookinc/` on each machine, never committed.

## Project — cookinc

Local-first cookie sync from Windows → Linux, for AI agents (Hermes, Claude Code, Cursor). End-to-end encrypted (AES-256-GCM), allowlist-only.

**Repo:** https://github.com/XDayonline/cookinc

---

## Architecture

```
Windows (source)                              Linux (sink)
═════════════════                            ═════════════
Chrome Extension                               cookinc-mcp
  → chrome.cookies API                          → HTTP /sync listener
  → POST localhost:19999                        → AES-256-GCM decrypt
     → cookinc.exe                              → Sidecar SQLite
       → encrypt                                → MCP server :9898
       → POST sync.example.com                  → Hermes / Claude / Cursor
```

- `cmd/cookinc/` — Windows daemon + Chrome extension bridge
- `cmd/cookinc-mcp/` — Linux sink + MCP server
- `internal/chrome/` — Chrome cookie reading (DPAPI, CDP, bridge)
- `internal/crypto/` — AES-256-GCM encrypt/decrypt
- `internal/config/` — YAML config loader (source/sink)
- `internal/protocol/` — Wire format (SyncEnvelope, Cookie)
- `internal/transport/` — HTTP sink server + replay defense
- `internal/mcp/` — MCP HTTP/stdio server
- `extension/` — Chrome Extension (MV3) for cookie export

---

## Quick start

### Windows (source)

```powershell
cd cookinc
go build -o cookinc.exe ./cmd/cookinc/

# Generate config
.\cookinc.exe init --sink-url https://sync.example.com --secret "your-32-char-min-secret" --allowlist github.com,vercel.com

# Start bridge (Chrome extension does the cookie reading)
.\cookinc.exe start

# Install the extension:
# 1. chrome://extensions → Developer mode ON
# 2. Load unpacked → select ./extension/
```

### Linux (sink)

```bash
cd cookinc
go build -o cookinc-mcp ./cmd/cookinc-mcp/

# Generate config + secret
./cookinc-mcp init --secret "same-32-char-secret-as-windows" --listen 0.0.0.0:9876

# Start
./cookinc-mcp start
```

---

## Building

```bash
# Windows (from Windows)
go build -o cookinc.exe ./cmd/cookinc/

# Linux native (from Linux)
go build -o cookinc-mcp ./cmd/cookinc-mcp/

# Cross-compile for Linux (from Windows — works thanks to pure-Go SQLite)
GOOS=linux GOARCH=amd64 go build -o cookinc-mcp ./cmd/cookinc-mcp/
```

No CGO required — `modernc.org/sqlite` is a pure-Go SQLite driver.

---

## Security rules

### NEVER commit

- `source.yaml` / `sink.yaml` — configs contain `shared_secret`
- `*.db` / `cookies-plain.json` — cookie databases
- `*.pem` / `*.key` / `*.crt` — TLS keys
- `cookinc.exe` / `cookinc-mcp` — binaries
- Any file containing `shared_secret`, `Bearer`, `token`, API keys, session tokens, JWTs, passwords
- `.env` files

### .gitignore covers

- `*.exe`, `*.db`, `.cookinc/`, `cookies-plain.json`, `cookinc-mcp`, `/bin/`, `/dist/`

### If you accidentally commit a secret

```bash
# 1. Rotate the secret immediately (generate new one)
# 2. Remove from history:
git filter-branch --force --index-filter "git rm --cached --ignore-unmatch path/to/secret-file" --prune-empty -- HEAD
git push --force
# 3. Tell all contributors to re-clone
```

---

## Config files (local only, never committed)

**Windows source** — `~/.config/cookinc/source.yaml`:
```yaml
sink:
  url: "https://sync.example.com"
security:
  shared_secret: "your-32-char-min-secret"
allowlist:
  domains:
    - github.com
peer:
  hostname: MyPC
watch:
  interval: "5s"
```

**Linux sink** — `~/.config/cookinc/sink.yaml`:
```yaml
listen:
  addr: "0.0.0.0:9876"
security:
  shared_secret: "same-secret-as-windows"
mcp:
  enabled: true
  addr: "127.0.0.1:9898"
```

---

## Key files

| File | Role |
|------|------|
| `cmd/cookinc/main.go` | CLI entry point |
| `cmd/cookinc/start_windows.go` | Bridge server (Windows only) |
| `cmd/cookinc/commands.go` | `init` + `status` commands |
| `internal/chrome/bridge.go` | HTTP server receiving cookies from extension |
| `internal/chrome/crypto_windows.go` | DPAPI decrypt + AES-128/256 + v10/v11/v20 |
| `internal/chrome/read_windows.go` | WindowsReader (DB + CDP) |
| `internal/chrome/copy_windows.go` | DB copy with retry (handles Chrome lock) |
| `internal/chrome/cdp.go` | Chrome DevTools Protocol reader |
| `internal/config/config.go` | YAML loader + defaults |
| `internal/crypto/crypto.go` | AES-256-GCM encrypt/decrypt |
| `internal/transport/transport.go` | HTTP sink + replay defense |
| `extension/manifest.json` | Chrome Extension MV3 manifest |
| `extension/background.js` | Service worker — reads cookies, POSTs to bridge |

---

## Chrome 127+ (App-Bound Encryption)

Chrome ≥127 on Windows encrypts cookies with App-Bound Encryption (ABE). Direct DPAPI decryption of v20 cookies is **not possible** from an external process — the key is managed by the Chrome Elevation Service.

**The solution:** the Chrome Extension uses the native `chrome.cookies` API, which Chrome internally decrypts. The extension pushes plaintext cookies to the local bridge server.

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `gopkg.in/yaml.v3` | Config parsing |
| `modernc.org/sqlite` | Pure-Go SQLite (no CGO) |
| `github.com/gorilla/websocket` | CDP WebSocket client |

No external service dependencies. Runs fully locally.

---

## Contributing

1. Fork + clone
2. `go build ./cmd/... && go vet ./...`
3. Keep diffs minimal — fix one thing per commit
4. Never commit binaries, secrets, or personal data
5. Test on both Windows and Linux targets
6. Update `.gitignore` if adding new local config paths
