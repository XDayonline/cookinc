<p align="center">
  <img src="https://img.shields.io/badge/status-pre--release-yellow" alt="Status">
  <img src="https://img.shields.io/badge/license-MIT-blue" alt="License">
  <img src="https://img.shields.io/badge/Windows→Linux-cross--platform-purple" alt="Cross-platform">
  <img src="https://img.shields.io/badge/MCP-ready-green" alt="MCP">
</p>

# cookinc 🍪🔗

**Local-first session sync for personal AI agents.**
Cross-platform, encrypted, allowlist-only.

Your AI agents (Hermes, Claude Code, Cursor, OpenCode) run on a Linux VPS.
You browse on your Windows machine.
cookinc keeps your Chrome sessions in sync — **from Windows → Linux** — so your agents wake up authenticated on every site you already are.

No cloud. No per-site re-auth ceremony. No infrastructure lock-in.

## The problem

Every AI agent that browses the web, scrapes a site, or calls an API behind your login has to re-authenticate. Every. Single. Time.

Existing tools lock you into one OS, one transport, or one cloud:

| Tool | Source | Target | Transport | Agent-ready |
|------|--------|--------|-----------|-------------|
| **Agent Cookie** | macOS | macOS | Tailscale | ❌ (no MCP) |
| **CookieCloud** | Browser ext. | Self-hosted | HTTP | ❌ (humans only) |
| **Browserbase** | Chrome | Cloud | Cloud API | ❌ (cloud dep.) |
| **AgentAuth/n8n** | Chrome | n8n vault | n8n plugin | ❌ (locked to n8n) |
| **cookinc** | **Windows** | **Linux** | **Any** | **✅ MCP native** |

cookinc is the **only tool built for the Windows → Linux agent workflow**.

## How it works

```
Windows (your daily driver)           Linux VPS (your agent)
══════════════════════════            ═══════════════════════

Chrome cookies change
(fsnotify on Cookies SQLite)
       │
       ▼
cookinc daemon                           cookinc-mcp
  ├─ Decrypt via DPAPI                    ├─ HTTP /sync listener
  ├─ Filter by allowlist                  ├─ AES-256-GCM decrypt
  ├─ AES-256-GCM seal                     ├─ Re-encrypt for Linux Chrome
  └─ POST /sync ─────────────────────►    ├─ Write to Cookies SQLite
                                          └─ MCP server on :9898
                                               │
                                               ▼
                                        Hermes / Claude / Cursor
                                        query: get_cookies("github.com")
```

Three delivery surfaces — pick what fits your agent:

1. **Chrome SQLite** — cookies written to the real Linux Chrome profile. Any unmodified tool (curl, yt-dlp, a browser-driving agent) reads them automatically.
2. **MCP** — agents query via Model Context Protocol. `get_cookies("github.com")` returns the session. Hermes, Claude Code, Cursor all speak MCP.
3. **Sidecar** — `~/.cookinc/cookies.json` for scripts or one-liners.

## Quick start

```bash
# On Windows (source):
cookinc init --allowlist github.com,vercel.com,x.com
cookinc start

# On Linux VPS (sink):
cookinc-mcp init --pair-url http://windows-pc:9876
cookinc-mcp start

# On Hermes, wire the MCP tool:
hermes config set mcp.cookinc "http://127.0.0.1:9898"
```

## Ethics & safety by design

- **Allowlist-only.** No "sync everything" mode. You explicitly list every domain whose cookies leave your Windows machine. Empty allowlist = nothing synced.
- **No cloud.** Cookies never touch a third-party server. You control the transport (HTTP over Tailscale, SSH tunnel, cloudflared, LAN).
- **End-to-end encrypted.** AES-256-GCM between source and sink. Wrong key = rejected payload.
- **Sink-side blocklist.** Defense in depth. Even a compromised source can't push cookies for blocked domains.
- **One-way.** Source → sink only. No reverse sync. Your agent never writes cookies back to your daily driver.

## Use cases

- **Hermes agent** on a Linux VPS that needs to browse the web as you
- **Claude Code** running automated PR reviews using your GitHub session
- **Cursor agent** scraping documentation behind login walls
- **Web automation** scripts that need to stay logged into SaaS tools
- **yt-dlp, gallery-dl** and other CLI tools reading cookies from real Chrome profile

## Status

Pre-release. Working on V1 — Chrome cookies Windows→Linux with allowlist + MCP.

### Roadmap

- **V1** — Chrome cookies, HTTP transport, allowlist, MCP server
- **V1.1** — File watcher (fsnotify), auto-sync on cookie change
- **V2** — Pairing (X25519 key exchange), persistent replay defense
- **V2.1** — LocalStorage/IndexedDB sync
- **V2.2** — Firefox support

## Why "cookinc"?

cookie + sync = **cookinc**. Also sounds like "cooking" — because you're cooking up sessions for your agents. 🍪

## License

MIT. Inspired by [agentcookie](https://github.com/mvanhorn/agentcookie).

---

<p align="center">
  <sub>Built for the Windows → Linux agent pipeline. Your daily driver stays yours. Your agents stay authenticated.</sub>
</p>
