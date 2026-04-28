package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"biene/internal/session"
)

// compactRequest is the JSON body for POST /api/sessions/{id}/compact.
// Instructions is optional; when set, it's appended to the summarizer
// prompt so the user can steer what to focus on.
type compactRequest struct {
	Instructions string `json:"instructions,omitempty"`
}

// compactResponse discriminates on `status`:
//
//   - "compacted": the summarizer ran; `message` carries the marker
//     DisplayMessage that the renderer should render in chat history.
//   - "no_op": the conversation was already short enough; nothing
//     happened, the renderer surfaces a friendly inline notice.
//
// Both shapes return HTTP 200 — neither is a failure. Real failures
// (busy session, summarizer error) still return 4xx/5xx with the usual
// {"error": ...} envelope.
type compactResponse struct {
	OK      bool                    `json:"ok"`
	Status  string                  `json:"status"`
	Message *session.DisplayMessage `json:"message,omitempty"`
	Reason  string                  `json:"reason,omitempty"`
}

// handleCompact triggers a manual compaction. The session refuses if a
// turn is already running, since the policy owner needs an idle history
// to safely rewrite api_messages.
func (s *Server) handleCompact(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}

	var req compactRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req) // empty body is fine
	}

	marker, err := sess.RunManualCompaction(r.Context(), s.cfg, strings.TrimSpace(req.Instructions))
	if errors.Is(err, session.ErrNoCompactionNeeded) {
		writeJSON(w, http.StatusOK, compactResponse{
			OK:     true,
			Status: "no_op",
			Reason: err.Error(),
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, compactResponse{
		OK:      true,
		Status:  "compacted",
		Message: marker,
	})
}
