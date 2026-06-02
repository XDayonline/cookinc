// Package transport defines the sync transport between cookinc source
// (Windows) and sink (Linux). The interface is kept small so multiple
// transports (HTTP, SSH tunnel, local file) can be implemented.
package transport

import (
	"net/http"
)

// SyncPayload is the encrypted byte stream sent from source to sink.
type SyncPayload []byte

// SourceClient sends encrypted payloads to the sink.
type SourceClient interface {
	// Send posts an encrypted payload to the sink's /sync endpoint.
	// Returns an error if the sink rejects the payload (wrong key,
	// replay, etc.).
	Send(payload SyncPayload) error
}

// SinkHandler handles incoming sync payloads on the sink side.
type SinkHandler interface {
	// HandleSync processes an incoming encrypted payload.
	// Returns HTTP status code and optional error message.
	HandleSync(payload SyncPayload) (int, string)
}

// HTTPSink wraps an http.Handler for the sink's /sync endpoint.
type HTTPSink struct {
	Handler func(payload SyncPayload) (int, string)
}

func (h *HTTPSink) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Max body size: 256 MB
	r.Body = http.MaxBytesReader(w, r.Body, 256<<20)
	buf := make([]byte, r.ContentLength)
	_, err := r.Body.Read(buf)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	status, msg := h.Handler(buf)
	if status != http.StatusOK {
		http.Error(w, msg, status)
		return
	}
	w.WriteHeader(http.StatusOK)
}
