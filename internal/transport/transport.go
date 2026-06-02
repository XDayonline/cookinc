// Package transport defines the sync transport between cookinc source
// (Windows) and sink (Linux). Multiple transports (HTTP, SSH tunnel,
// cloudflared) can be implemented.
package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/XDayonline/cookinc/internal/chrome"
	"github.com/XDayonline/cookinc/internal/crypto"
	"github.com/XDayonline/cookinc/internal/protocol"
)

// SyncPayload is the encrypted byte stream from source to sink.
type SyncPayload []byte

// SinkServer receives encrypted cookies from the Windows source,
// decrypts them, validates them, and writes them to the sidecar store.
type SinkServer struct {
	store     *chrome.Sidecar
	secretKey []byte
	addr      string

	// Replay defense
	tracker *SequenceTracker
}

// NewSinkServer creates a new sink server.
// secret is the shared secret (min 32 chars).
func NewSinkServer(store *chrome.Sidecar, secret string, addr string) *SinkServer {
	key := crypto.DeriveKeyFromSecret(secret)
	return &SinkServer{
		store:     store,
		secretKey: key,
		addr:      addr,
		tracker:   NewSequenceTracker(),
	}
}

// ListenAndServe starts the HTTP server and blocks.
func (s *SinkServer) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/sync", s.handleSync)
	mux.HandleFunc("/health", s.handleHealth)

	server := &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("cookinc-mcp: listening on %s", s.addr)
	return server.ListenAndServe()
}

// Addr returns the configured listen address.
func (s *SinkServer) Addr() string {
	return s.addr
}

func (s *SinkServer) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Enforce body size limit (256 MB)
	r.Body = http.MaxBytesReader(w, r.Body, 256<<20)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("sync: read body: %v", err)
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	// Step 1: Decrypt
	plaintext, err := crypto.Decrypt(s.secretKey, body)
	if err != nil {
		log.Printf("sync: decrypt failed (wrong key?): %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Step 2: Unmarshal envelope
	var env protocol.SyncEnvelope
	if err := json.Unmarshal(plaintext, &env); err != nil {
		log.Printf("sync: unmarshal: %v", err)
		http.Error(w, "bad envelope", http.StatusBadRequest)
		return
	}

	// Step 3: Check protocol version
	if env.ProtocolVersion != protocol.ProtocolVersion {
		msg := fmt.Sprintf("unsupported protocol version %d (expected %d)", env.ProtocolVersion, protocol.ProtocolVersion)
		log.Printf("sync: %s", msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	// Step 4: Replay defense
	if !s.tracker.Accept(env.SourceHostname, env.Sequence) {
		log.Printf("sync: replay rejected from %s (seq %d)", env.SourceHostname, env.Sequence)
		http.Error(w, "sequence conflict", http.StatusConflict)
		return
	}

	// Step 5: Write to sidecar
	if err := s.store.WriteCookies(env.Cookies); err != nil {
		log.Printf("sync: write error: %v", err)
		http.Error(w, "write error", http.StatusInternalServerError)
		return
	}

	log.Printf("sync: %d cookies from %s (seq %d)", len(env.Cookies), env.SourceHostname, env.Sequence)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":     "ok",
		"count":      len(env.Cookies),
		"source":     env.SourceHostname,
		"sequence":   env.Sequence,
	})
}

func (s *SinkServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.Count()
	if err != nil {
		count = -1
	}
	lastUpdated, _ := s.store.LastUpdated()

	json.NewEncoder(w).Encode(map[string]any{
		"status":       "ok",
		"cookie_count": count,
		"last_updated": lastUpdated.Format(time.RFC3339),
		"listeners":    s.addr,
	})
}

// SequenceTracker provides in-memory replay defense.
type SequenceTracker struct {
	sequences map[string]int64
}

func NewSequenceTracker() *SequenceTracker {
	return &SequenceTracker{sequences: make(map[string]int64)}
}

// Accept returns true if the sequence is valid (strictly greater than
// the last seen sequence for the source).
func (t *SequenceTracker) Accept(sourceHostname string, seq int64) bool {
	last, exists := t.sequences[sourceHostname]
	if exists && seq <= last {
		return false
	}
	t.sequences[sourceHostname] = seq
	return true
}
