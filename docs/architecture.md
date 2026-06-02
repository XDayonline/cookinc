# cookinc architecture

## Overview

cookinc is a two-component system:

1. **cookinc** — runs on Windows (source), watches Chrome cookies, pushes to sink
2. **cookinc-mcp** — runs on Linux (sink), receives cookies, serves MCP

## Modules

```
cmd/
  cookinc/           Windows daemon (cobra CLI)
    main.go
    cmd/
      init.go        Create source.yaml
      start.go       Start file watcher + sync loop
      status.go      Show sync health
  cookinc-mcp/       Linux MCP server
    main.go
    cmd/
      init.go        Create sink.yaml
      start.go       Start HTTP listener + MCP server
      status.go      Show sync health

internal/
  chrome/
    read.go          Interface + Windows DPAPI reader
    write.go         Interface + Linux AES writer
    paths.go         Platform-specific Chrome path detection
  config/
    config.go        SourceConfig / SinkConfig structs + YAML loading
  crypto/
    crypto.go        AES-256-GCM encrypt/decrypt
  protocol/
    envelope.go      SyncEnvelope JSON wire format
  transport/
    transport.go     SourceClient / SinkHandler interfaces
    http.go          HTTP transport implementation
  mcp/
    server.go        MCP protocol server for AI agents
    tools.go         Tool definitions (get_cookies, list_domains, etc.)
```

## Sync lifecycle

1. **Watch** (Windows): fsnotify on Chrome's Cookies SQLite file
2. **Debounce** (Windows): wait 2s after last change (batch writes)
3. **Read + decrypt** (Windows): open SQLite, read cookies, decrypt values with DPAPI
4. **Filter** (Windows): keep only allowlisted domains
5. **Encrypt** (Windows): seal SyncEnvelope with AES-256-GCM
6. **Transport** (Windows→Linux): POST encrypted payload to sink URL
7. **Verify** (Linux): decrypt, check sequence for replay, validate protocol version
8. **Filter** (Linux): apply sink-side blocklist (optional defense in depth)
9. **Write** (Linux): re-encrypt with local AES key, write to Chrome SQLite
10. **MCP** (Linux): update sidecar DB for MCP queries

## Security boundaries

| Layer | Protection |
|-------|-----------|
| OS user account | File mode 0600 on keys, config |
| Chrome cookies | DPAPI (Windows), AES (Linux) |
| Transport | AES-256-GCM over user-chosen channel |
| Key derivation | X25519 + HKDF (pairing) or SHA-256 (shared secret) |
| Replay defense | Monotonic sequence number per source |
| Allowlist | Source-side, domain-level, empty = sync nothing |
| Blocklist | Sink-side, defense in depth against compromised source |
