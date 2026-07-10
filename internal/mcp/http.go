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

	mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(dashboardHTML))
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

var dashboardHTML = `<!DOCTYPE html>
<html lang="fr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Cookinc Dashboard</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#111;color:#e5e7eb;padding:20px}
h1{font-size:22px;margin-bottom:4px;color:#fbbf24}
.sub{font-size:13px;color:#6b7280;margin-bottom:20px}
.stats{display:flex;gap:12px;margin-bottom:20px}
.stat{background:#1f2937;border-radius:10px;padding:14px 20px;flex:1}
.stat .num{font-size:28px;font-weight:700;color:#fbbf24}
.stat .label{font-size:12px;color:#9ca3af}
table{width:100%;border-collapse:collapse}
th{text-align:left;font-size:12px;color:#6b7280;padding:8px 12px;border-bottom:1px solid #374151}
td{padding:10px 12px;border-bottom:1px solid #1f2937;font-size:14px}
td a{color:#60a5fa;text-decoration:none}
td a:hover{text-decoration:underline}
.dot{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:6px}
.dot.on{background:#22c55e}
.dot.off{background:#6b7280}
.updated{font-size:12px;color:#6b7280;margin-top:20px;text-align:center}
</style>
</head>
<body>
<h1>🍪 Cookinc</h1>
<div class="sub">Tableau de bord des sessions synchronisées</div>

<div class="stats">
  <div class="stat">
    <div class="num" id="totalCookies">—</div>
    <div class="label">Cookies</div>
  </div>
  <div class="stat">
    <div class="num" id="totalDomains">—</div>
    <div class="label">Domaines</div>
  </div>
  <div class="stat">
    <div class="num" id="lastUpdate">—</div>
    <div class="label">Dernière sync</div>
  </div>
</div>

<table>
<thead><tr><th>Domaine</th><th>Cookies</th><th>Dernière activité</th></tr></thead>
<tbody id="rows"></tbody>
</table>

<div class="updated" id="refreshed">Chargement...</div>

<script>
async function refresh() {
  try {
    const [health, domainsResp] = await Promise.all([
      fetch('/health').then(r=>r.json()),
      fetch('/api/domains').then(r=>r.json())
    ]);
    document.getElementById('totalCookies').textContent = health.mcp.cookie_count;
    document.getElementById('totalDomains').textContent = domainsResp.count;
    document.getElementById('lastUpdate').textContent = health.mcp.last_updated || '';
    document.getElementById('refreshed').textContent = 'Mis à jour: ' + new Date().toLocaleTimeString('fr-FR');

    const tbody = document.getElementById('rows');
    tbody.innerHTML = '';
    for (const d of domainsResp.domains) {
      const cookieResp = await fetch('/api/cookies?domain='+encodeURIComponent(d));
      const cookieData = await cookieResp.json();
      const count = cookieData.count || 0;
      const tr = document.createElement('tr');
      tr.innerHTML = '<td><a href="/api/cookies?domain='+encodeURIComponent(d)+'" target="_blank">'+d+'</a></td><td>'+count+'</td><td><span class="dot '+(count>0?'on':'off')+'"></span></td>';
      tbody.appendChild(tr);
    }
  } catch(e) {
    document.getElementById('refreshed').textContent = 'Erreur: ' + e.message;
  }
}
refresh();
setInterval(refresh, 10000);
</script>
</body>
</html>`
