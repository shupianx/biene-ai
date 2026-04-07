package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"biene/internal/config"
)

// Server is the biene HTTP server.
type Server struct {
	cfg  *config.Config
	mgr  *SessionManager
	host string
	port int
}

// Options configures the server on creation.
type Options struct {
	Host          string
	Port          int
	Config        *config.Config
	WorkspaceRoot string // defaults to "workspace" (relative to cwd)
}

// New creates a Server and initialises the session manager.
func New(opts Options) (*Server, error) {
	host := opts.Host
	if host == "" {
		host = "127.0.0.1"
	}

	workspaceRoot := opts.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = "workspace"
	}

	mgr := NewSessionManager(workspaceRoot, opts.Config)
	mgr.Init()

	return &Server{
		cfg:  opts.Config,
		mgr:  mgr,
		host: host,
		port: opts.Port,
	}, nil
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	// Sessions CRUD
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	mux.HandleFunc("POST /api/sessions", s.handleCreateSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("POST /api/sessions/{id}/settings", s.handleUpdateSession)
	mux.HandleFunc("GET /api/sessions/{id}/history", s.handleSessionHistory)
	mux.HandleFunc("GET /api/sessions/{id}/file", s.handleSessionFile)

	// Per-session chat + events
	mux.HandleFunc("GET /api/sessions/{id}/events", s.handleChatEvents)
	mux.HandleFunc("POST /api/sessions/{id}/send", s.handleChatSend)
	mux.HandleFunc("POST /api/sessions/{id}/interrupt", s.handleChatInterrupt)
	mux.HandleFunc("POST /api/sessions/{id}/permission", s.handlePermission)

	// Global config
	mux.HandleFunc("GET /api/config", s.handleConfig)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	fmt.Printf("biene-core listening on http://%s\n", addr)
	return http.ListenAndServe(addr, corsMiddleware(mux))
}

// corsMiddleware adds permissive CORS headers for local development.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── Response helpers ──────────────────────────────────────────────────────

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response: {"error": msg}.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// lookupSession is a helper used by per-session handlers.
func (s *Server) lookupSession(w http.ResponseWriter, r *http.Request) *Session {
	id := r.PathValue("id")
	sess := s.mgr.Get(id)
	if sess == nil {
		writeError(w, http.StatusNotFound, "session not found")
	}
	return sess
}
