package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"biene/internal/prompt"
	"biene/internal/tools"
)

// handleListSessions returns metadata for all sessions.
// GET /api/sessions
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.List())
}

// createSessionRequest is the body for POST /api/sessions.
type createSessionRequest struct {
	Name        string               `json:"name"`
	Permissions *tools.PermissionSet `json:"permissions"`
	Profile     *prompt.AgentProfile `json:"profile"`
}

// handleCreateSession creates a new agent session with its own workspace.
// POST /api/sessions
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Name = "New Agent"
	}
	if req.Name == "" {
		req.Name = "New Agent"
	}
	req.Name = strings.TrimSpace(req.Name)
	if s.mgr.NameTaken(req.Name, "") {
		writeError(w, http.StatusConflict, "agent name already exists")
		return
	}
	perms := tools.PermissionSet{}
	if req.Permissions != nil {
		perms = *req.Permissions
	}
	profile := prompt.DefaultProfile()
	if req.Profile != nil {
		profile = *req.Profile
	}
	sess, err := s.mgr.Create(req.Name, perms, profile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, sess.Meta())
}

type updateSessionRequest struct {
	Name        *string              `json:"name"`
	Permissions *tools.PermissionSet `json:"permissions"`
	Profile     *prompt.AgentProfile `json:"profile"`
}

// handleUpdateSession updates mutable session settings.
// POST /api/sessions/{id}/settings
func (s *Server) handleUpdateSession(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}

	var req updateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	meta := sess.Meta()
	name := meta.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		if s.mgr.NameTaken(name, sess.ID) {
			writeError(w, http.StatusConflict, "agent name already exists")
			return
		}
	}

	perms := meta.Permissions
	if req.Permissions != nil {
		perms = *req.Permissions
	}
	profile := meta.Profile
	if req.Profile != nil {
		profile = *req.Profile
	}
	meta, err := sess.UpdateSettings(name, perms, profile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, meta)
}

// handleDeleteSession removes a session and deletes its workspace from disk.
// DELETE /api/sessions/{id}
func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.mgr.Delete(id) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleSessionHistory returns the full display history for a session.
// Used by clients reconnecting after a page refresh or tab reopen.
// GET /api/sessions/{id}/history
func (s *Server) handleSessionHistory(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}
	writeJSON(w, http.StatusOK, sess.SnapshotHistory())
}
