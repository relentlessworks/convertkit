package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// wantsJSON checks if the client wants JSON output.
func wantsJSON(r *http.Request) bool {
	if r.URL.Query().Get("format") == "json" {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json")
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeText writes a plain text response.
func writeText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprint(w, body)
}

// writeError writes an error response in the appropriate format.
func writeError(w http.ResponseWriter, r *http.Request, status int, msg, hint string) {
	if wantsJSON(r) {
		writeJSON(w, status, map[string]string{
			"error": msg,
			"hint":  hint,
		})
		return
	}
	writeText(w, status, fmt.Sprintf("error: %s | hint: %s\n", msg, hint))
}

// writeRecord writes a single record in the appropriate format.
func writeRecord(w http.ResponseWriter, r *http.Request, v interface{}) {
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, v)
		return
	}
	// For plain text, output as key=value pairs
	writeText(w, http.StatusOK, recordToText(v))
}

// recordToText converts a struct/map to plain text key=value format.
func recordToText(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v\n", v)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return string(b) + "\n"
	}
	var parts []string
	for k, val := range m {
		parts = append(parts, fmt.Sprintf("%s=%v", k, val))
	}
	return strings.Join(parts, " ") + "\n"
}
