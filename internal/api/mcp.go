package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/relentlessworks/convertkit/internal/converter"
	"github.com/relentlessworks/convertkit/internal/model"
)

// MCPRequest is a JSON-RPC 2.0 request.
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse is a JSON-RPC 2.0 response.
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST with JSON-RPC 2.0")
		return
	}

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, MCPResponse{
			JSONRPC: "2.0",
			Error:   &MCPError{Code: -32700, Message: "Parse error"},
		})
		return
	}

	resp := s.handleMCPMethod(r, &req)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMCPMethod(r *http.Request, req *MCPRequest) MCPResponse {
	switch req.Method {
	case "initialize":
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "convertkit",
					"version": "0.1.0",
				},
			},
		}

	case "tools/list":
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"tools": s.mcpTools(),
			},
		}

	case "tools/call":
		return s.handleMCPToolCall(r, req)

	default:
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
		}
	}
}

func (s *Server) mcpTools() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "convert",
			"description": "Convert data between formats (json, yaml, toml, csv, xml, properties, jsonl, env)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"data": map[string]interface{}{
						"type":        "string",
						"description": "The data to convert",
					},
					"from": map[string]interface{}{
						"type":        "string",
						"description": "Source format: json, yaml, toml, csv, xml, properties, jsonl, env",
					},
					"to": map[string]interface{}{
						"type":        "string",
						"description": "Target format: json, yaml, toml, csv, xml, properties, jsonl, env",
					},
				},
				"required": []string{"data", "from", "to"},
			},
		},
		{
			"name":        "list_formats",
			"description": "List all supported data formats",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "get_history",
			"description": "Get recent conversion history (requires auth token)",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token": map[string]interface{}{
						"type":        "string",
						"description": "Bearer token for authentication",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Max number of records to return (default 10)",
					},
				},
				"required": []string{"token"},
			},
		},
	}
}

func (s *Server) handleMCPToolCall(r *http.Request, req *MCPRequest) MCPResponse {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32602, Message: "Invalid params"},
		}
	}

	switch params.Name {
	case "convert":
		data, _ := params.Arguments["data"].(string)
		from, _ := params.Arguments["from"].(string)
		to, _ := params.Arguments["to"].(string)

		if data == "" || from == "" || to == "" {
			return MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &MCPError{Code: -32602, Message: "data, from, and to are required"},
			}
		}

		output, err := converter.Convert([]byte(data), converter.Format(from), converter.Format(to))
		if err != nil {
			return MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &MCPError{Code: -32603, Message: err.Error()},
			}
		}

		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": string(output),
					},
				},
			},
		}

	case "list_formats":
		formats := converter.SupportedFormats()
		var lines []string
		for _, f := range formats {
			lines = append(lines, fmt.Sprintf("%s: %s", f, converter.FormatDescription(f)))
		}
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": strings.Join(lines, "\n"),
					},
				},
			},
		}

	case "get_history":
		token, _ := params.Arguments["token"].(string)
		if token == "" {
			return MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &MCPError{Code: -32602, Message: "token is required"},
			}
		}

		wsHandle, err := s.authSvc.ValidateToken(token)
		if err != nil {
			return MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &MCPError{Code: -32603, Message: "invalid token"},
			}
		}

		limit := 10
		if l, ok := params.Arguments["limit"].(float64); ok {
			limit = int(l)
		}

		records, err := s.store.ListConversions(wsHandle, limit)
		if err != nil {
			return MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &MCPError{Code: -32603, Message: err.Error()},
			}
		}

		var lines []string
		for _, rec := range records {
			lines = append(lines, fmt.Sprintf("handle=%s from=%s to=%s created_at=%s", rec.Handle, rec.FromFmt, rec.ToFmt, rec.CreatedAt))
		}
		if len(lines) == 0 {
			lines = append(lines, "no conversions yet")
		}

		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": strings.Join(lines, "\n"),
					},
				},
			},
		}

	default:
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPError{Code: -32601, Message: fmt.Sprintf("Unknown tool: %s", params.Name)},
		}
	}
}

// readBody is a helper to read the full request body.
func readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

// ensure model is used
var _ = model.GenerateHandle
