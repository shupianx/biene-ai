package server

import (
	"encoding/json"
	"net/http"

	"biene/internal/permission"
)

// permissionResponse is the body for POST /api/sessions/{id}/permission.
type permissionResponse struct {
	RequestID string `json:"request_id"`
	Decision  string `json:"decision"` // "allow" | "always" | "deny"
}

// handlePermission resolves a pending permission request for a session.
// POST /api/sessions/{id}/permission
func (s *Server) handlePermission(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}

	var resp permissionResponse
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var decision permission.Decision
	switch resp.Decision {
	case "allow":
		decision = permission.DecisionAllow
	case "always":
		decision = permission.DecisionAlwaysAllow
	default:
		decision = permission.DecisionDeny
	}

	meta, err := sess.ResolvePermission(resp.RequestID, decision)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, meta)
}
