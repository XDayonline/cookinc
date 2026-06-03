// Package chrome provides CDP-based cookie reading for Chrome 127+
// where App-Bound Encryption prevents direct DB decryption.
//
// Build constraint: windows only (domainMatches requires Windows-specific files).
//
//go:build windows

package chrome

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/XDayonline/cookinc/internal/protocol"
)

// cdpReadCookies reads cookies via Chrome DevTools Protocol.
// Requires Chrome to be started with --remote-debugging-port=9222.
func cdpReadCookies(allowlist []string) ([]protocol.Cookie, error) {
	if len(allowlist) == 0 {
		return nil, nil
	}

	wsURL, err := cdpGetWebSocketURL("http://localhost:9222")
	if err != nil {
		return nil, fmt.Errorf("chrome: CDP connect: %w", err)
	}

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("chrome: CDP ws dial: %w", err)
	}
	defer c.Close()

	// Send Network.getCookies
	cmd := map[string]any{
		"id":     1,
		"method": "Network.getCookies",
		"params": map[string]any{},
	}
	if err := c.WriteJSON(cmd); err != nil {
		return nil, fmt.Errorf("chrome: CDP send: %w", err)
	}

	// Read response
	var resp struct {
		ID     int `json:"id"`
		Result struct {
			Cookies []struct {
				Name     string `json:"name"`
				Value    string `json:"value"`
				Domain   string `json:"domain"`
				Path     string `json:"path"`
				Expires  float64 `json:"expires"`
				Secure   bool    `json:"secure"`
				HTTPOnly bool    `json:"httpOnly"`
				SameSite string  `json:"sameSite"`
				Priority string  `json:"priority"`
			} `json:"cookies"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := c.ReadJSON(&resp); err != nil {
		return nil, fmt.Errorf("chrome: CDP response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("chrome: CDP error: %s", resp.Error.Message)
	}

	var cookies []protocol.Cookie
	for _, ck := range resp.Result.Cookies {
		if !domainMatches(ck.Domain, allowlist) {
			continue
		}
		cookies = append(cookies, protocol.Cookie{
			HostKey:    ck.Domain,
			Name:       ck.Name,
			Value:      ck.Value,
			Path:       ck.Path,
			ExpiresUTC: int64(ck.Expires),
			IsSecure:   ck.Secure,
			IsHTTPOnly: ck.HTTPOnly,
			SameSite:   sameSiteToInt(ck.SameSite),
		})
	}

	return cookies, nil
}

// cdpGetWebSocketURL discovers the first page WebSocket debugger URL
// from Chrome's HTTP discovery endpoint.
func cdpGetWebSocketURL(baseURL string) (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(baseURL + "/json")
	if err != nil {
		return "", fmt.Errorf("discover: %w", err)
	}
	defer resp.Body.Close()

	var targets []struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return "", fmt.Errorf("parse targets: %w", err)
	}

	for _, t := range targets {
		if t.WebSocketDebuggerURL != "" {
			return t.WebSocketDebuggerURL, nil
		}
	}
	return "", fmt.Errorf("no debuggable target found")
}

// cdpCheckPort checks if the Chrome DevTools Protocol port is responding.
// cdpCheckPort checks if the Chrome DevTools Protocol port is responding.
func cdpCheckPort(baseURL string) bool {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(baseURL + "/json/version")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
