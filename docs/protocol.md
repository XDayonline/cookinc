# cookinc protocol v1

Wire format between cookinc source (Windows) and sink (Linux).

## Layers, outside to inside

1. **Transport.** HTTP POST to `/sync` on the sink machine. The request body is the sealed AEAD ciphertext. `Content-Type: application/octet-stream`.
2. **Authenticated encryption.** AES-256-GCM. Key is either derived from a shared secret (via SHA-256, secret must be ≥ 32 bytes) or from a future X25519 pairing exchange. Every message carries a fresh 12-byte nonce prepended to the ciphertext. Wrong-secret payloads are rejected by the AEAD tag check.
3. **Envelope.** Inside the seal, the plaintext is a JSON `SyncEnvelope`.

## SyncEnvelope (JSON)

```json
{
  "protocol_version": 1,
  "source_hostname": "my-windows-pc",
  "sequence": 1747432817123456789,
  "cookies": [
    {
      "host_key": ".github.com",
      "name": "user_session",
      "value": "abc123...",
      "path": "/",
      "expires_utc": 17789688170000000,
      "is_secure": true,
      "is_httponly": true
    }
  ]
}
```

### Field reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `protocol_version` | int | yes | Must equal sink's compiled-in version. Bumping is a breaking change. |
| `source_hostname` | string | yes | Source's hostname. Used for replay-defense bookkeeping. |
| `sequence` | int64 | yes | Monotonically increasing per source. Sink rejects ≤ last seen. |
| `cookies` | array | yes | Decrypted cookies matching Chrome's cookie table schema. |

Each cookie object mirrors Chrome's SQLite `cookies` table columns. Values are
strings/ints — never raw bytes.

## Sink validation order

1. Decrypt the body with the configured shared/paired key. Reject `401 Unauthorized` on failure.
2. JSON-unmarshal the envelope. Reject `400 Bad Request` on failure.
3. Check `protocol_version == 1`. Reject `400` otherwise.
4. Check `sequence` against in-memory tracker for `source_hostname`. Reject `409 Conflict` if ≤ last seen.
5. Filter cookies against sink-side blocklist (if configured). Dropped cookies are logged.
6. Write remaining cookies to Chrome SQLite. Re-encrypt each value with the local Chrome Safe Storage key.

## Replay defense (v1)

Sequence tracker is in-memory only. Sink restart clears it; a captured payload
could be replayed once after restart. Persistent replay defense is planned
for v2 (SQLite-backed sequence store).

## Allowlist (source side, mandatory)

The source reads `source.yaml` and ONLY sends cookies whose `host_key` matches
an entry in `allowlist.domains`. An empty allowlist means nothing is synced.
Pattern matching is simple prefix/suffix — e.g. `github.com` matches
`.github.com`, `www.github.com`, etc.

## Blocklist (sink side, optional)

The sink can optionally read a `blocklist.yaml` independent of the source.
If the source pushes a cookie for a blocked domain, the sink drops it
silently. This is defense in depth — even a fully compromised source cannot
force cookies for blocked domains onto the sink.

## Versioning

- v1 is the current wire format. Source and sink must both speak it.
- Future versions: bump `protocol_version` and update both sides.
- New optional fields may be added under v1 first, then graduate to required in v2.
