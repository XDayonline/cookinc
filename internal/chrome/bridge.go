// Package chrome provides the local cookie ingestion server.
// The Chrome extension POSTs cookies here; cookinc encrypts and forwards.
package chrome

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/XDayonline/cookinc/internal/crypto"
	"github.com/XDayonline/cookinc/internal/protocol"
)

// BridgeConfig is served to the Chrome extension via /config endpoint.
type BridgeConfig struct {
	Allowlist []string `json:"allowlist"`
	Interval  int      `json:"interval"`
}

// BridgeServer receives cookies from the Chrome extension,
// encrypts them, and forwards to the sink URL.
type BridgeServer struct {
	addr       string
	allowlist  []string
	secretKey  []byte
	sinkURL    string
	hostname   string
	client     *http.Client
}

// NewBridgeServer creates a new bridge server.
func NewBridgeServer(addr string, allowlist []string, secret, sinkURL, hostname string) *BridgeServer {
	return &BridgeServer{
		addr:      addr,
		allowlist: allowlist,
		secretKey: crypto.DeriveKeyFromSecret(secret),
		sinkURL:   sinkURL,
		hostname:  hostname,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// ListenAndServe starts the bridge HTTP server and blocks.
func (s *BridgeServer) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/config", s.handleConfig)
	mux.HandleFunc("/cookies", s.handleCookies)

	log.Printf("cookinc: bridge server on %s", s.addr)
	log.Printf("cookinc: install extension from ./extension/ in chrome://extensions (dev mode)")
	log.Printf("cookinc: allowlist: %v", s.allowlist)

	return (&http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
	}).ListenAndServe()
}

func (s *BridgeServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(BridgeConfig{
		Allowlist: s.allowlist,
		Interval:  5,
	})
}

func (s *BridgeServer) handleCookies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	var rawCookies []struct {
		Domain   string  `json:"domain"`
		Name     string  `json:"name"`
		Value    string  `json:"value"`
		Path     string  `json:"path"`
		Secure   bool    `json:"secure"`
		HTTPOnly bool    `json:"httpOnly"`
		Expires  float64 `json:"expirationDate"`
		SameSite string  `json:"sameSite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&rawCookies); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	cookies := make([]protocol.Cookie, 0, len(rawCookies))
	for _, c := range rawCookies {
		cookies = append(cookies, protocol.Cookie{
			HostKey:    c.Domain,
			Name:       c.Name,
			Value:      c.Value,
			Path:       c.Path,
			ExpiresUTC: int64(c.Expires),
			IsSecure:   c.Secure,
			IsHTTPOnly: c.HTTPOnly,
			SameSite:   sameSiteToInt(c.SameSite),
		})
	}

	if len(cookies) == 0 {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "count": 0})
		return
	}

	// Forward to sink
	env := protocol.NewEnvelope(s.hostname, cookies)
	plaintext, err := json.Marshal(env)
	if err != nil {
		http.Error(w, "marshal: "+err.Error(), http.StatusInternalServerError)
		return
	}
	sealed, err := crypto.Encrypt(s.secretKey, plaintext)
	if err != nil {
		http.Error(w, "encrypt: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := s.client.Post(s.sinkURL+"/sync", "application/octet-stream", bytes.NewReader(sealed))
	if err != nil {
		log.Printf("bridge: forward to sink: %v", err)
		http.Error(w, "sink unreachable", http.StatusBadGateway)
		return
	}
	resp.Body.Close()

	log.Printf("bridge: forwarded %d cookies to %s", len(cookies), s.sinkURL)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "count": len(cookies)})
}
