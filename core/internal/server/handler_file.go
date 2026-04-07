package server

import (
	"net/http"
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

	resolvedPath, _, err := resolveWorkspacePath(sess.WorkDir, requestedPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	http.ServeFile(w, r, resolvedPath)
}
