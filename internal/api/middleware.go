package api

import (
	"net/http"
	"strings"

	"github.com/relentlessworks/convertkit/internal/auth"
	"github.com/relentlessworks/convertkit/internal/store"
)

// extractToken gets the bearer token from the Authorization header.
func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// authMiddleware wraps a handler with token authentication.
func authMiddleware(authSvc *auth.AuthService, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			writeError(w, r, http.StatusUnauthorized, "missing auth token", "call POST /auth/request with email to get an OTP, then POST /auth/verify to get a bearer token")
			return
		}

		wsHandle, err := authSvc.ValidateToken(token)
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "invalid or expired token", "call POST /auth/request with email to get a new OTP, then POST /auth/verify")
			return
		}

		r.Header.Set("X-Workspace", wsHandle)
		next(w, r)
	}
}

// getWorkspace extracts the workspace handle from the request.
func getWorkspace(r *http.Request) string {
	return r.Header.Get("X-Workspace")
}

// Server holds all dependencies for the API.
type Server struct {
	store   *store.Store
	authSvc *auth.AuthService
}

// NewServer creates a new API server.
func NewServer(s *store.Store, a *auth.AuthService) *Server {
	return &Server{store: s, authSvc: a}
}

// Routes returns the HTTP handler with all routes registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/help", s.handleHelp)
	mux.HandleFunc("/.well-known/agent.md", s.handleHelp)
	mux.HandleFunc("/formats", s.handleFormats)
	mux.HandleFunc("/auth/request", s.handleAuthRequest)
	mux.HandleFunc("/auth/verify", s.handleAuthVerify)

	// Authenticated endpoints
	mux.HandleFunc("/convert", authMiddleware(s.authSvc, s.handleConvert))
	mux.HandleFunc("/history", authMiddleware(s.authSvc, s.handleHistory))
	mux.HandleFunc("/history/", authMiddleware(s.authSvc, s.handleHistoryItem))
	mux.HandleFunc("/audit", authMiddleware(s.authSvc, s.handleAudit))
	mux.HandleFunc("/workspace", authMiddleware(s.authSvc, s.handleWorkspace))

	// MCP endpoint
	mux.HandleFunc("/mcp", s.handleMCP)

	// Root
	mux.HandleFunc("/", s.handleRoot)

	return mux
}
