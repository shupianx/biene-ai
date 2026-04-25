package session

import (
	"encoding/json"
	"log/slog"
	"strings"

	"biene/internal/api"
	"biene/internal/prompt"
	"biene/internal/tools"
)

// ── api.Message serialization ─────────────────────────────────────────────
// api.Message.Content is an interface slice; we use a custom JSON envelope.

type storedAPIMsg struct {
	Role    string        `json:"role"`
	Content []storedBlock `json:"content"`
}

type storedBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Signature string          `json:"signature,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Path      string          `json:"path,omitempty"`
	MediaType string          `json:"media_type,omitempty"`
}

func marshalAPIMessage(m api.Message) (json.RawMessage, error) {
	s := storedAPIMsg{Role: m.Role}
	for _, block := range m.Content {
		var b storedBlock
		switch v := block.(type) {
		case api.TextBlock:
			b = storedBlock{Type: "text", Text: v.Text}
		case api.ReasoningBlock:
			b = storedBlock{Type: "reasoning", Text: v.Text, Signature: v.Signature}
		case api.ToolUseBlock:
			b = storedBlock{Type: "tool_use", ID: v.ID, Name: v.Name, Input: v.Input}
		case api.ToolResultBlock:
			b = storedBlock{Type: "tool_result", ToolUseID: v.ToolUseID, Content: v.Content, IsError: v.IsError}
		case api.ImageBlock:
			b = storedBlock{Type: "image", Path: v.Path, MediaType: v.MediaType}
		default:
			continue
		}
		s.Content = append(s.Content, b)
	}
	return json.Marshal(s)
}

func unmarshalAPIMessage(raw json.RawMessage) (api.Message, error) {
	var s storedAPIMsg
	if err := json.Unmarshal(raw, &s); err != nil {
		return api.Message{}, err
	}
	m := api.Message{Role: s.Role}
	for _, b := range s.Content {
		switch b.Type {
		case "text":
			m.Content = append(m.Content, api.TextBlock{Text: b.Text})
		case "reasoning":
			m.Content = append(m.Content, api.ReasoningBlock{Text: b.Text, Signature: b.Signature})
		case "tool_use":
			m.Content = append(m.Content, api.ToolUseBlock{ID: b.ID, Name: b.Name, Input: b.Input})
		case "tool_result":
			m.Content = append(m.Content, api.ToolResultBlock{ToolUseID: b.ToolUseID, Content: b.Content, IsError: b.IsError})
		case "image":
			m.Content = append(m.Content, api.ImageBlock{Path: b.Path, MediaType: b.MediaType})
		}
	}
	return m, nil
}

// ── Persistence helpers ───────────────────────────────────────────────────

func (s *Session) persistDisplayMessage(msg DisplayMessage) {
	if s.store == nil {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("marshal display msg", "session_id", s.ID, "msg_id", msg.ID, "err", err)
		return
	}
	if err := s.store.AppendDisplayMessage(msg.ID, data); err != nil {
		slog.Error("persist display msg", "session_id", s.ID, "msg_id", msg.ID, "err", err)
	}
}

func (s *Session) updatePersistedDisplayMessage(msg DisplayMessage) {
	if s.store == nil {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("marshal display msg update", "session_id", s.ID, "msg_id", msg.ID, "err", err)
		return
	}
	if err := s.store.UpdateDisplayMessage(msg.ID, data); err != nil {
		slog.Error("persist display msg update", "session_id", s.ID, "msg_id", msg.ID, "err", err)
	}
}

func (s *Session) persistMetaSnapshot(meta SessionMeta) {
	if s.store == nil {
		return
	}
	if err := s.store.SaveMeta(meta); err != nil {
		slog.Error("persist meta", "session_id", s.ID, "err", err)
	}
}

func (s *Session) persistAfterRun(newDisplay []DisplayMessage, apiMsgs []api.Message, meta SessionMeta) {
	if s.store == nil {
		return
	}
	for _, msg := range newDisplay {
		s.persistDisplayMessage(msg)
	}
	rawMsgs := make([]json.RawMessage, 0, len(apiMsgs))
	for _, m := range apiMsgs {
		raw, err := marshalAPIMessage(m)
		if err != nil {
			slog.Error("marshal api msg", "session_id", s.ID, "err", err)
			continue
		}
		rawMsgs = append(rawMsgs, raw)
	}
	if err := s.store.ReplaceAPIMessages(rawMsgs); err != nil {
		slog.Error("persist api messages", "session_id", s.ID, "err", err)
	}
	if err := s.store.SaveMeta(meta); err != nil {
		slog.Error("persist meta", "session_id", s.ID, "err", err)
	}
}

func (s *Session) persistPermissions(perms tools.PermissionSet) {
	s.mu.Lock()
	s.permissions = perms
	meta := s.metaLocked()
	persistedMeta := s.persistentMetaLocked()
	s.mu.Unlock()

	if s.store != nil {
		if err := s.store.SaveMeta(persistedMeta); err != nil {
			slog.Error("persist permissions", "session_id", s.ID, "err", err)
		}
	}

	s.notifyMetaChanged(meta)
}

func (s *Session) SetThinkingEnabled(enabled bool) (SessionMeta, error) {
	s.mu.Lock()
	if !s.thinkingAvailable {
		enabled = false
	}
	s.thinkingEnabled = enabled
	meta := s.metaLocked()
	persistedMeta := s.persistentMetaLocked()
	s.mu.Unlock()

	if s.store != nil {
		if err := s.store.SaveMeta(persistedMeta); err != nil {
			return SessionMeta{}, err
		}
	}
	s.notifyMetaChanged(meta)
	return meta, nil
}

func (s *Session) UpdateSettings(name string, perms tools.PermissionSet, profile prompt.AgentProfile) (SessionMeta, error) {
	name = strings.TrimSpace(name)
	profile = normalizeProfile(profile)
	toolMode := defaultToolModeForProfile(profile)

	s.mu.Lock()
	if name != "" {
		s.Name = name
	}
	s.permissions = perms
	s.profile = profile
	s.toolMode = toolMode
	s.checker.SetPermissions(perms)
	s.systemPrompt = prompt.Build(s.registry, s.WorkDir, s.profile, prompt.AgentIdentity{
		ID:      s.ID,
		Name:    s.Name,
		WorkDir: s.WorkDir,
	}, nil)
	meta := s.metaLocked()
	persistedMeta := s.persistentMetaLocked()
	s.mu.Unlock()

	if s.store != nil {
		if err := s.store.SaveMeta(persistedMeta); err != nil {
			return SessionMeta{}, err
		}
	}
	s.notifyMetaChanged(meta)
	return meta, nil
}
