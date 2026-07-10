// Package protocol defines the wire format between cookinc source (Windows)
// and sink (Linux). Each sync payload is a JSON envelope sealed inside
// AES-256-GCM with replay defense via sequence numbers.
package protocol

import "time"

// ProtocolVersion is the current wire format version.
const ProtocolVersion = 1

// SyncEnvelope is the plaintext JSON payload inside the AEAD seal.
// Field order matters for deterministic JSON encoding in tests.
type SyncEnvelope struct {
	ProtocolVersion int      `json:"protocol_version"`
	SourceHostname  string   `json:"source_hostname"`
	Sequence        int64    `json:"sequence"`
	Cookies         []Cookie `json:"cookies"`
}

// Cookie mirrors Chrome's cookies table fields.
// All values are plaintext (decrypted on source, to be re-encrypted on sink).
type Cookie struct {
	HostKey     string `json:"host_key"`
	Name        string `json:"name"`
	Value       string `json:"value"`
	Path        string `json:"path"`
	ExpiresUTC  int64  `json:"expires_utc,omitempty"`
	IsSecure    bool   `json:"is_secure"`
	IsHTTPOnly  bool   `json:"is_httponly"`
	Priority    int    `json:"priority,omitempty"`
	SameSite    int    `json:"samesite,omitempty"`
	SourcePort  int    `json:"source_port,omitempty"`
}

// NewEnvelope creates a sync envelope with current timestamp as sequence.
func NewEnvelope(hostname string, cookies []Cookie) SyncEnvelope {
	return SyncEnvelope{
		ProtocolVersion: ProtocolVersion,
		SourceHostname:  hostname,
		Sequence:        time.Now().UnixNano(),
		Cookies:         cookies,
	}
}
