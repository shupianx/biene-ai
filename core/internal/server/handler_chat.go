package server

import (
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"strings"
)

const maxMultipartMemory = 32 << 20

// handleChatEvents serves the SSE stream for a session.
// GET /api/sessions/{id}/events
func (s *Server) handleChatEvents(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	subID, events := sess.subscribeEvents()
	defer sess.unsubscribeEvents(subID)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-events:
			if !ok {
				return
			}
			writeSSE(w, frame)
		}
	}
}

// sendRequest is the JSON body for POST /api/sessions/{id}/send.
type sendRequest struct {
	Text            string `json:"text"`
	ClientMessageID string `json:"client_message_id,omitempty"`
}

type incomingInput struct {
	Text            string
	ClientMessageID string
	Files           []uploadedFile
}

// handleChatSend accepts either JSON text input or multipart text+files.
// POST /api/sessions/{id}/send
func (s *Server) handleChatSend(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}

	input, err := parseIncomingInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	attachments, err := storeUploadedFiles(sess.WorkDir, "uploads", input.Files)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sess.enqueueUserInput(input.Text, attachments, input.ClientMessageID)

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleChatInterrupt cancels the in-flight run for a session, if any.
// POST /api/sessions/{id}/interrupt
func (s *Server) handleChatInterrupt(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}

	sess.cancelCurrentQuery()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func parseIncomingInput(r *http.Request) (incomingInput, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return parseMultipartInput(r)
	}

	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return incomingInput{}, errors.New("invalid request body")
	}
	req.Text = strings.TrimSpace(req.Text)
	req.ClientMessageID = strings.TrimSpace(req.ClientMessageID)
	if req.Text == "" {
		return incomingInput{}, errors.New("provide text and/or files")
	}
	return incomingInput{
		Text:            req.Text,
		ClientMessageID: req.ClientMessageID,
	}, nil
}

func parseMultipartInput(r *http.Request) (incomingInput, error) {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		return incomingInput{}, errors.New("invalid multipart form")
	}

	text := strings.TrimSpace(r.FormValue("text"))
	clientMessageID := strings.TrimSpace(r.FormValue("client_message_id"))
	headers := r.MultipartForm.File["files"]
	files := make([]uploadedFile, 0, len(headers))
	for _, header := range headers {
		file, err := readMultipartFile(header)
		if err != nil {
			return incomingInput{}, err
		}
		files = append(files, file)
	}

	if text == "" && len(files) == 0 {
		return incomingInput{}, errors.New("provide text and/or files")
	}
	return incomingInput{
		Text:            text,
		ClientMessageID: clientMessageID,
		Files:           files,
	}, nil
}

func readMultipartFile(header *multipart.FileHeader) (uploadedFile, error) {
	f, err := header.Open()
	if err != nil {
		return uploadedFile{}, err
	}
	defer f.Close()
	return readUploadedFile(header.Filename, f)
}
