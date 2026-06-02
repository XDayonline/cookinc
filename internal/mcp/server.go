// Package mcp implements the Model Context Protocol (MCP) server
// for cookinc. Agents query cookies via standard MCP tools.
//
// MCP protocol: https://spec.modelcontextprotocol.io/
// JSON-RPC 2.0 over stdio (standard for Claude Desktop, Cursor)
// or over HTTP (for Hermes custom integration).
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

// Server implements the MCP stdio server.
type Server struct {
	handler *ToolHandler
	reader  *bufio.Scanner
	writer  *json.Encoder
}

// ToolHandler handles MCP tool calls.
type ToolHandler struct {
	GetCookies    func(domain string) (any, error)
	ListDomains   func() (any, error)
	SyncStatus    func() (any, error)
}

// New creates a new MCP server with the given tool handler.
func New(handler *ToolHandler) *Server {
	return &Server{
		handler: handler,
		reader:  bufio.NewScanner(os.Stdin),
		writer:  json.NewEncoder(os.Stdout),
	}
}

// Run starts the MCP server over stdio. Blocks forever.
func (s *Server) Run() error {
	log.Println("cookinc-mcp: MCP server started (stdio mode)")
	for s.reader.Scan() {
		line := s.reader.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var req json.RawMessage
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(nil, -32700, "Parse error: invalid JSON")
			continue
		}

		s.handleMessage(req)
	}
	return s.reader.Err()
}

// MCP Protocol message types
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErrorObject    `json:"error,omitempty"`
}

type ErrorObject struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErrorObject    `json:"error,omitempty"`
}

// Tool definition for tools/list
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// input schemas
var getCookiesSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"domain": map[string]any{
			"type":        "string",
			"description": "Domain to get cookies for (e.g. 'github.com')",
		},
	},
	"required": []string{"domain"},
}

func (s *Server) handleMessage(raw json.RawMessage) {
	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		s.sendError(nil, -32700, "Parse error")
		return
	}

	switch msg.Method {
	case "initialize":
		s.handleInitialize(msg.ID)
	case "notifications/initialized":
		// No response needed for notifications
	case "notifications/cancelled":
		// No response needed
	case "tools/list":
		s.handleToolsList(msg.ID)
	case "tools/call":
		s.handleToolCall(msg.ID, msg.Params)
	default:
		s.sendError(msg.ID, -32601, fmt.Sprintf("Method not found: %s", msg.Method))
	}
}

func (s *Server) handleInitialize(id json.RawMessage) {
	resp := map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "cookinc-mcp",
			"version": "1.0.0",
		},
	}
	s.sendResult(id, resp)
}

func (s *Server) handleToolsList(id json.RawMessage) {
	tools := []ToolDefinition{
		{
			Name:        "get_cookies",
			Description: "Get Chrome cookies for a specific domain. Returns cookie names, values, paths, and metadata.",
			InputSchema: getCookiesSchema,
		},
		{
			Name:        "list_domains",
			Description: "List all domains that have synced cookies available.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}},
		},
		{
			Name:        "sync_status",
			Description: "Get the current sync status: last update time, cookie count, and server health.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}},
		},
	}
	s.sendResult(id, map[string]any{"tools": tools})
}

func (s *Server) handleToolCall(id json.RawMessage, params json.RawMessage) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		s.sendError(id, -32602, "Invalid params")
		return
	}

	switch call.Name {
	case "get_cookies":
		var args struct {
			Domain string `json:"domain"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil || args.Domain == "" {
			s.sendError(id, -32602, "Missing or invalid 'domain' argument")
			return
		}
		result, err := s.handler.GetCookies(args.Domain)
		if err != nil {
			s.sendError(id, -32603, err.Error())
			return
		}
		s.sendResult(id, result)

	case "list_domains":
		result, err := s.handler.ListDomains()
		if err != nil {
			s.sendError(id, -32603, err.Error())
			return
		}
		s.sendResult(id, result)

	case "sync_status":
		result, err := s.handler.SyncStatus()
		if err != nil {
			s.sendError(id, -32603, err.Error())
			return
		}
		s.sendResult(id, result)

	default:
		s.sendError(id, -32601, fmt.Sprintf("Tool not found: %s", call.Name))
	}
}

func (s *Server) sendResult(id json.RawMessage, result any) {
	data, _ := json.Marshal(result)
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  data,
	}
	s.writer.Encode(resp)
}

func (s *Server) sendError(id json.RawMessage, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ErrorObject{
			Code:    code,
			Message: message,
		},
	}
	s.writer.Encode(resp)
}
