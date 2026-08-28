package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/relentlessworks/convertkit/internal/converter"
	"github.com/relentlessworks/convertkit/internal/model"
)

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, r, http.StatusNotFound, "not found", "GET /help for the full API manual")
		return
	}
	writeText(w, http.StatusOK, "convertkit — agentic-first data format conversion service\nGET /help for instructions\n")
}

func (s *Server) handleHelp(w http.ResponseWriter, r *http.Request) {
	help := `convertkit — agentic-first data format conversion service

DESCRIPTION
  Convert data between JSON, YAML, TOML, CSV, XML, Properties, JSONL, and Env formats.
  The agent IS the interface. No UI, no SDK. Just plain HTTP.

AUTH FLOW
  1. POST /auth/request  body: email=user@example.com
     → Sends OTP to email (or logs to stderr in dev mode)
  2. POST /auth/verify   body: email=user@example.com code=123456
     → Returns: token=abc123...
  3. Use token in all subsequent requests:
     Authorization: Bearer abc123...

ENDPOINTS

  POST /convert?from=json&to=yaml
    Body: raw data in the source format
    Response: converted data in the target format
    Example: curl -X POST -d '{"name":"test","value":42}' "http://localhost:7700/convert?from=json&to=yaml"
    → name: test
      value: 42

  GET /formats
    Lists all supported formats with descriptions.

  GET /history?limit=10
    Lists recent conversions (newest first).

  GET /history/{handle}
    Retrieves a specific conversion by its handle.

  GET /workspace
    Shows current workspace info.

  GET /audit?limit=20
    Shows audit log entries for the current workspace.

SUPPORTED FORMATS
  json, yaml, toml, csv, xml, properties, jsonl, env

RESPONSE FORMAT
  Plain text by default. Add Accept: application/json or ?format=json for JSON.
  Errors include a hint: error: message | hint: what to do next

MCP
  POST /mcp — Model Context Protocol JSON-RPC 2.0 endpoint
  Tools: convert, list_formats, get_history
`
	writeText(w, http.StatusOK, help)
}

func (s *Server) handleFormats(w http.ResponseWriter, r *http.Request) {
	formats := converter.SupportedFormats()
	if wantsJSON(r) {
		result := make([]map[string]string, 0, len(formats))
		for _, f := range formats {
			result = append(result, map[string]string{
				"format":      string(f),
				"description": converter.FormatDescription(f),
			})
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"formats": result})
		return
	}

	var buf strings.Builder
	for _, f := range formats {
		buf.WriteString(fmt.Sprintf("format=%s desc=%s\n", f, converter.FormatDescription(f)))
	}
	writeText(w, http.StatusOK, buf.String())
}

func (s *Server) handleAuthRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST")
		return
	}

	email := r.FormValue("email")
	if email == "" {
		// Try JSON body
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			email = body["email"]
		}
	}
	if email == "" {
		writeError(w, r, http.StatusBadRequest, "email is required", "send email as form field or JSON body: {\"email\":\"user@example.com\"}")
		return
	}

	wsHandle, err := s.authSvc.RequestOTP(email)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to send OTP", "check that the email is valid")
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":    "ok",
			"message":   "OTP sent to email (or logged to stderr in dev mode)",
			"workspace": wsHandle,
		})
		return
	}
	writeText(w, http.StatusOK, fmt.Sprintf("status=ok message=OTP sent to %s (check stderr in dev mode) workspace=%s\n", email, wsHandle))
}

func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST")
		return
	}

	email := r.FormValue("email")
	code := r.FormValue("code")
	if email == "" || code == "" {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if email == "" {
				email = body["email"]
			}
			if code == "" {
				code = body["code"]
			}
		}
	}
	if email == "" || code == "" {
		writeError(w, r, http.StatusBadRequest, "email and code are required", "send both email and code as form fields or JSON body")
		return
	}

	token, err := s.authSvc.VerifyOTP(email, code)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, err.Error(), "request a new OTP via POST /auth/request")
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]string{
			"token":     token.Token,
			"workspace": token.Workspace,
		})
		return
	}
	writeText(w, http.StatusOK, fmt.Sprintf("token=%s workspace=%s\n", token.Token, token.Workspace))
}

func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST with from and to query params")
		return
	}

	fromFmt := r.URL.Query().Get("from")
	toFmt := r.URL.Query().Get("to")

	if fromFmt == "" {
		// Try form field or header
		fromFmt = r.FormValue("from")
	}
	if toFmt == "" {
		toFmt = r.FormValue("to")
	}
	if fromFmt == "" {
		fromFmt = r.Header.Get("X-From-Format")
	}
	if toFmt == "" {
		toFmt = r.Header.Get("X-To-Format")
	}

	if fromFmt == "" || toFmt == "" {
		writeError(w, r, http.StatusBadRequest, "from and to parameters are required", "use query params: ?from=json&to=yaml, or form fields, or X-From-Format/X-To-Format headers")
		return
	}

	if !converter.IsValidFormat(fromFmt) {
		writeError(w, r, http.StatusBadRequest, fmt.Sprintf("unsupported source format: %s", fromFmt), "use one of: json, yaml, toml, csv, xml, properties, jsonl, env")
		return
	}
	if !converter.IsValidFormat(toFmt) {
		writeError(w, r, http.StatusBadRequest, fmt.Sprintf("unsupported target format: %s", toFmt), "use one of: json, yaml, toml, csv, xml, properties, jsonl, env")
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "failed to read request body", "ensure you're sending the data to convert in the request body")
		return
	}
	if len(body) == 0 {
		writeError(w, r, http.StatusBadRequest, "request body is empty", "send the data to convert in the request body")
		return
	}

	// Convert
	output, err := converter.Convert(body, converter.Format(fromFmt), converter.Format(toFmt))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error(), "check that your input is valid and the formats are compatible")
		return
	}

	// Save to history
	wsHandle := getWorkspace(r)
	handle := model.GenerateHandle("conv")
	rec := &model.ConversionRecord{
		Handle:    handle,
		FromFmt:   fromFmt,
		ToFmt:     toFmt,
		Input:     string(body),
		Output:    string(output),
		CreatedAt: time.Now(),
	}
	s.store.SaveConversion(wsHandle, rec)
	s.store.AddAudit(model.AuditEntry{
		Handle: wsHandle,
		Action: "convert",
		Detail: fmt.Sprintf("%s → %s (handle=%s)", fromFmt, toFmt, handle),
	})

	// Return converted data directly (not wrapped in key=value)
	// The output IS the response body
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Handle", handle)
	w.WriteHeader(http.StatusOK)
	w.Write(output)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	wsHandle := getWorkspace(r)
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := fmt.Sscanf(l, "%d", &limit); n != 1 || err != nil {
			limit = 20
		}
	}

	records, err := s.store.ListConversions(wsHandle, limit)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to list history", "try again or contact support")
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"conversions": records,
			"count":       len(records),
		})
		return
	}

	if len(records) == 0 {
		writeText(w, http.StatusOK, "no conversions yet\n")
		return
	}

	var buf strings.Builder
	for _, rec := range records {
		buf.WriteString(fmt.Sprintf("handle=%s from=%s to=%s created_at=%s\n", rec.Handle, rec.FromFmt, rec.ToFmt, rec.CreatedAt.Format(time.RFC3339)))
	}
	writeText(w, http.StatusOK, buf.String())
}

func (s *Server) handleHistoryItem(w http.ResponseWriter, r *http.Request) {
	wsHandle := getWorkspace(r)
	handle := strings.TrimPrefix(r.URL.Path, "/history/")
	if handle == "" {
		writeError(w, r, http.StatusBadRequest, "handle is required", "use GET /history/{handle} with a valid conversion handle")
		return
	}

	rec, err := s.store.GetConversion(wsHandle, handle)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "conversion not found", "check the handle or list history with GET /history")
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, rec)
		return
	}

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("handle=%s from=%s to=%s created_at=%s\n", rec.Handle, rec.FromFmt, rec.ToFmt, rec.CreatedAt.Format(time.RFC3339)))
	buf.WriteString(fmt.Sprintf("--- input (%s) ---\n%s\n", rec.FromFmt, rec.Input))
	buf.WriteString(fmt.Sprintf("--- output (%s) ---\n%s\n", rec.ToFmt, rec.Output))
	writeText(w, http.StatusOK, buf.String())
}

func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	wsHandle := getWorkspace(r)
	ws, err := s.store.GetWorkspaceByHandle(wsHandle)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "workspace not found", "your token may be invalid")
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, ws)
		return
	}

	writeText(w, http.StatusOK, fmt.Sprintf("handle=%s name=%s email=%s plan=%s created_at=%s\n",
		ws.Handle, ws.Name, ws.Email, ws.Plan, ws.CreatedAt.Format(time.RFC3339)))
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	wsHandle := getWorkspace(r)
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := fmt.Sscanf(l, "%d", &limit); n != 1 || err != nil {
			limit = 20
		}
	}

	entries, err := s.store.ListAudit(wsHandle, limit)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to list audit log", "try again")
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"entries": entries,
			"count":   len(entries),
		})
		return
	}

	if len(entries) == 0 {
		writeText(w, http.StatusOK, "no audit entries\n")
		return
	}

	var buf strings.Builder
	for _, e := range entries {
		buf.WriteString(fmt.Sprintf("id=%d action=%s detail=%s timestamp=%s\n", e.ID, e.Action, e.Detail, e.Timestamp.Format(time.RFC3339)))
	}
	writeText(w, http.StatusOK, buf.String())
}
