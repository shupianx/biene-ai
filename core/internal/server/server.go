package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"biene/internal/config"
	"biene/internal/prompt"
	"biene/internal/session"
)

const authHeaderName = "X-Biene-Token"

// Server is the biene HTTP server.
type Server struct {
	cfg          *config.Config
	mgr          *session.SessionManager
	host         string
	port         int
	authToken    string
	httpServer   *http.Server
	shutdownOnce sync.Once
}

// Options configures the server on creation.
type Options struct {
	Host          string
	Port          int
	Config        *config.Config
	WorkspaceRoot string // defaults to "workspace" (relative to cwd)
	AuthToken     string
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

	_ = prompt.CurrentCatalog()

	mgr := session.NewSessionManager(workspaceRoot, opts.Config)
	mgr.Init()

	return &Server{
		cfg:       opts.Config,
		mgr:       mgr,
		host:      host,
		port:      opts.Port,
		authToken: strings.TrimSpace(opts.AuthToken),
	}, nil
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST /api/admin/shutdown", s.handleShutdown)

	// Sessions CRUD
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	mux.HandleFunc("GET /api/sessions/ws", s.handleSessionListWebSocket)
	mux.HandleFunc("POST /api/sessions", s.handleCreateSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("POST /api/sessions/{id}/settings", s.handleUpdateSession)
	mux.HandleFunc("GET /api/sessions/{id}/history", s.handleSessionHistory)
	mux.HandleFunc("GET /api/sessions/{id}/file", s.handleSessionFile)

	// Per-session chat + realtime events
	mux.HandleFunc("GET /api/sessions/{id}/ws", s.handleChatWebSocket)
	mux.HandleFunc("POST /api/sessions/{id}/send", s.handleChatSend)
	mux.HandleFunc("POST /api/sessions/{id}/thinking", s.handleThinking)
	mux.HandleFunc("POST /api/sessions/{id}/interrupt", s.handleChatInterrupt)
	mux.HandleFunc("POST /api/sessions/{id}/permission", s.handlePermission)
	mux.HandleFunc("POST /api/sessions/{id}/skills/install", s.handleSessionInstallSkill)
	mux.HandleFunc("DELETE /api/sessions/{id}/skills/{skill_id}", s.handleSessionUninstallSkill)

	// Global config
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("POST /api/config", s.handleUpdateConfig)
	mux.HandleFunc("GET /api/skills", s.handleListSkills)
	mux.HandleFunc("POST /api/skills/config", s.handleUpdateSkillsConfig)
	mux.HandleFunc("POST /api/skills/import", s.handleImportSkills)
	mux.HandleFunc("DELETE /api/skills/{id}", s.handleDeleteSkill)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	fmt.Printf("biene-core listening on http://%s\n", addr)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(authMiddleware(s.authToken, mux)),
	}
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// corsMiddleware adds permissive CORS headers for local development.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Biene-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authMiddleware(token string, next http.Handler) http.Handler {
	token = strings.TrimSpace(token)
	if token == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get(authHeaderName)
		if provided == "" {
			provided = r.URL.Query().Get("token")
		}
		if !tokenMatches(token, provided) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func tokenMatches(expected, provided string) bool {
	if expected == "" || provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

func (s *Server) handleShutdown(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()
}

func (s *Server) Shutdown(ctx context.Context) error {
	var shutdownErr error
	s.shutdownOnce.Do(func() {
		if s.httpServer != nil {
			shutdownErr = s.httpServer.Shutdown(ctx)
		}
		s.mgr.Close()
	})
	return shutdownErr
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
func (s *Server) lookupSession(w http.ResponseWriter, r *http.Request) *session.Session {
	id := r.PathValue("id")
	sess := s.mgr.Get(id)
	if sess == nil {
		writeError(w, http.StatusNotFound, "session not found")
	}
	return sess
}
