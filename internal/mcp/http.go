package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// HTTPServer wraps the MCP handler behind an HTTP server.
// Useful for Hermes integration and curl testing.
func NewHTTPServer(handler *ToolHandler, addr string) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}

		var req Message
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}`, http.StatusBadRequest)
			return
		}

		// Process the message and write the response
		w.Header().Set("Content-Type", "application/json")
		processAndRespond(w, req, handler)
	})

	// Convenience endpoints for quick testing
	mux.HandleFunc("/api/domains", func(w http.ResponseWriter, r *http.Request) {
		domains, err := handler.ListDomains()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(domains)
	})

	mux.HandleFunc("/api/cookies", func(w http.ResponseWriter, r *http.Request) {
		domain := r.URL.Query().Get("domain")
		if domain == "" {
			http.Error(w, "missing domain query param", http.StatusBadRequest)
			return
		}
		cookies, err := handler.GetCookies(domain)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cookies)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status, err := handler.SyncStatus()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"mcp":    status,
		})
	})

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func processAndRespond(w http.ResponseWriter, req Message, handler *ToolHandler) {
	resp := processMessage(&req, handler)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func processMessage(req *Message, handler *ToolHandler) Response {
	switch req.Method {
	case "initialize":
		result := map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "cookinc-mcp", "version": "1.0.0"},
		}
		data, _ := json.Marshal(result)
		return Response{JSONRPC: "2.0", ID: req.ID, Result: data}

	case "tools/list":
		tools := []ToolDefinition{
			{
				Name:        "get_cookies",
				Description: "Get Chrome cookies for a specific domain.",
				InputSchema: getCookiesSchema,
			},
			{
				Name:        "list_domains",
				Description: "List all domains with synced cookies.",
				InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}},
			},
			{
				Name:        "sync_status",
				Description: "Get current sync status.",
				InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}},
			},
		}
		data, _ := json.Marshal(map[string]any{"tools": tools})
		return Response{JSONRPC: "2.0", ID: req.ID, Result: data}

	case "tools/call":
		var call struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &call); err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &ErrorObject{Code: -32602, Message: "Invalid params"}}
		}

		switch call.Name {
		case "get_cookies":
			var args struct{ Domain string `json:"domain"` }
			if err := json.Unmarshal(call.Arguments, &args); err != nil || args.Domain == "" {
				return Response{JSONRPC: "2.0", ID: req.ID, Error: &ErrorObject{Code: -32602, Message: "Missing domain"}}
			}
			result, err := handler.GetCookies(args.Domain)
			if err != nil {
				return Response{JSONRPC: "2.0", ID: req.ID, Error: &ErrorObject{Code: -32603, Message: err.Error()}}
			}
			data, _ := json.Marshal(result)
			return Response{JSONRPC: "2.0", ID: req.ID, Result: data}

		case "list_domains":
			result, err := handler.ListDomains()
			if err != nil {
				return Response{JSONRPC: "2.0", ID: req.ID, Error: &ErrorObject{Code: -32603, Message: err.Error()}}
			}
			data, _ := json.Marshal(result)
			return Response{JSONRPC: "2.0", ID: req.ID, Result: data}

		case "sync_status":
			result, err := handler.SyncStatus()
			if err != nil {
				return Response{JSONRPC: "2.0", ID: req.ID, Error: &ErrorObject{Code: -32603, Message: err.Error()}}
			}
			data, _ := json.Marshal(result)
			return Response{JSONRPC: "2.0", ID: req.ID, Result: data}

		default:
			return Response{JSONRPC: "2.0", ID: req.ID, Error: &ErrorObject{Code: -32601, Message: "Tool not found"}}
		}

	default:
		// For notifications, return empty success
		if req.Method == "notifications/initialized" || req.Method == "notifications/cancelled" {
			return Response{JSONRPC: "2.0", ID: req.ID}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Error: &ErrorObject{Code: -32601, Message: "Method not found"}}
	}
}
