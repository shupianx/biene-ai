package server

import (
	"net/http"

	"tinte/internal/session"
)

// handleSessionFile serves a file from the session workspace.
// GET /api/sessions/{id}/file?path=uploads/foo.txt
func (s *Server) handleSessionFile(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}

	requestedPath := r.URL.Query().Get("path")
	if requestedPath == "" {
		writeError(w, http.StatusBadRequest, "missing path query parameter")
		return
	}

	resolvedPath, _, err := session.ResolveWorkspacePath(sess.WorkDir, requestedPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	http.ServeFile(w, r, resolvedPath)
}

// handleSessionAsset serves a chat-level asset (e.g. user-uploaded image)
// from the reserved .tinte/assets/user/ directory. Path traversal outside
// that directory is rejected.
// GET /api/sessions/{id}/assets/{path...}
func (s *Server) handleSessionAsset(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}

	requestedPath := r.PathValue("path")
	if requestedPath == "" {
		writeError(w, http.StatusBadRequest, "missing asset path")
		return
	}

	resolvedPath, err := session.ResolveSessionAssetPath(sess.WorkDir, requestedPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	http.ServeFile(w, r, resolvedPath)
}
