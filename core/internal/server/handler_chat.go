package server

import (
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"strings"

	"biene/internal/api"
	"biene/internal/session"
)

const maxMultipartMemory = 32 << 20

// sendRequest is the JSON body for POST /api/sessions/{id}/send.
type sendRequest struct {
	Text            string `json:"text"`
	ClientMessageID string `json:"client_message_id,omitempty"`
	ThinkingEnabled *bool  `json:"thinking_enabled,omitempty"`
}

type thinkingRequest struct {
	Enabled *bool `json:"enabled"`
}

type incomingInput struct {
	Text            string
	ClientMessageID string
	ThinkingEnabled *bool
	Files           []session.UploadedFile
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

	var (
		fileUploads  []session.UploadedFile
		imageUploads []session.UploadedFile
	)
	for _, f := range input.Files {
		if session.IsImageMediaType(f.MediaType) {
			imageUploads = append(imageUploads, f)
		} else {
			fileUploads = append(fileUploads, f)
		}
	}

	attachments, err := session.StoreUploadedFiles(sess.WorkDir, fileUploads)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var imageBlocks []api.ImageBlock
	for _, img := range imageUploads {
		att, err := session.StoreUploadedImage(sess.WorkDir, img)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		attachments = append(attachments, att)
		imageBlocks = append(imageBlocks, api.ImageBlock{
			Path:      att.Path,
			MediaType: att.MediaType,
		})
	}

	if input.ThinkingEnabled != nil {
		if _, err := sess.SetThinkingEnabled(*input.ThinkingEnabled); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	sess.EnqueueUserInput(input.Text, attachments, imageBlocks, input.ClientMessageID)

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleThinking updates the current thinking toggle for a session.
// POST /api/sessions/{id}/thinking
func (s *Server) handleThinking(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}

	var req thinkingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	meta, err := sess.SetThinkingEnabled(*req.Enabled)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

// handleChatInterrupt cancels the in-flight run for a session, if any.
// POST /api/sessions/{id}/interrupt
func (s *Server) handleChatInterrupt(w http.ResponseWriter, r *http.Request) {
	sess := s.lookupSession(w, r)
	if sess == nil {
		return
	}

	sess.CancelCurrentQuery()
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
		ThinkingEnabled: req.ThinkingEnabled,
	}, nil
}

func parseMultipartInput(r *http.Request) (incomingInput, error) {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		return incomingInput{}, errors.New("invalid multipart form")
	}

	text := strings.TrimSpace(r.FormValue("text"))
	clientMessageID := strings.TrimSpace(r.FormValue("client_message_id"))
	thinkingEnabled, err := parseOptionalBool(strings.TrimSpace(r.FormValue("thinking_enabled")))
	if err != nil {
		return incomingInput{}, err
	}
	headers := r.MultipartForm.File["files"]
	files := make([]session.UploadedFile, 0, len(headers))
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
		ThinkingEnabled: thinkingEnabled,
		Files:           files,
	}, nil
}

func parseOptionalBool(value string) (*bool, error) {
	if value == "" {
		return nil, nil
	}
	switch strings.ToLower(value) {
	case "true":
		parsed := true
		return &parsed, nil
	case "false":
		parsed := false
		return &parsed, nil
	default:
		return nil, errors.New("invalid thinking_enabled value")
	}
}

func readMultipartFile(header *multipart.FileHeader) (session.UploadedFile, error) {
	f, err := header.Open()
	if err != nil {
		return session.UploadedFile{}, err
	}
	defer f.Close()
	uploaded, err := session.ReadUploadedFile(header.Filename, f)
	if err != nil {
		return session.UploadedFile{}, err
	}
	uploaded.MediaType = strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	if uploaded.MediaType == "" || uploaded.MediaType == "application/octet-stream" {
		uploaded.MediaType = http.DetectContentType(uploaded.Data)
	}
	return uploaded, nil
}
