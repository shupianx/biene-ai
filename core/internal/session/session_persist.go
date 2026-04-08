package session

import (
	"encoding/json"
	"log"
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
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

func marshalAPIMessage(m api.Message) (json.RawMessage, error) {
	s := storedAPIMsg{Role: m.Role}
	for _, block := range m.Content {
		var b storedBlock
		switch v := block.(type) {
		case api.TextBlock:
			b = storedBlock{Type: "text", Text: v.Text}
		case api.ToolUseBlock:
			b = storedBlock{Type: "tool_use", ID: v.ID, Name: v.Name, Input: v.Input}
		case api.ToolResultBlock:
			b = storedBlock{Type: "tool_result", ToolUseID: v.ToolUseID, Content: v.Content, IsError: v.IsError}
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
		case "tool_use":
			m.Content = append(m.Content, api.ToolUseBlock{ID: b.ID, Name: b.Name, Input: b.Input})
		case "tool_result":
			m.Content = append(m.Content, api.ToolResultBlock{ToolUseID: b.ToolUseID, Content: b.Content, IsError: b.IsError})
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
		log.Printf("persist display msg %s: %v", msg.ID, err)
		return
	}
	if err := s.store.AppendDisplayMessage(msg.ID, data); err != nil {
		log.Printf("persist display msg %s: %v", msg.ID, err)
	}
}

func (s *Session) updatePersistedDisplayMessage(msg DisplayMessage) {
	if s.store == nil {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("marshal display msg update %s: %v", msg.ID, err)
		return
	}
	if err := s.store.UpdateDisplayMessage(msg.ID, data); err != nil {
		log.Printf("persist display msg update %s: %v", msg.ID, err)
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
			log.Printf("persist api msg: %v", err)
			continue
		}
		rawMsgs = append(rawMsgs, raw)
	}
	if err := s.store.ReplaceAPIMessages(rawMsgs); err != nil {
		log.Printf("persist api messages: %v", err)
	}
	if err := s.store.SaveMeta(meta); err != nil {
		log.Printf("persist meta: %v", err)
	}
}

func (s *Session) persistPermissions(perms tools.PermissionSet) {
	s.mu.Lock()
	s.permissions = perms
	meta := s.persistentMetaLocked()
	s.mu.Unlock()

	if s.store == nil {
		return
	}
	if err := s.store.SaveMeta(meta); err != nil {
		log.Printf("persist permissions for %s: %v", s.ID, err)
	}
}

func (s *Session) UpdateSettings(name string, perms tools.PermissionSet, profile prompt.AgentProfile) (SessionMeta, error) {
	name = strings.TrimSpace(name)
	profile = normalizeProfile(profile)

	s.mu.Lock()
	if name != "" {
		s.Name = name
	}
	s.permissions = perms
	s.profile = profile
	s.checker.SetPermissions(perms)
	s.systemPrompt = prompt.Build(s.registry, s.WorkDir, s.profile)
	meta := s.persistentMetaLocked()
	s.mu.Unlock()

	if s.store != nil {
		if err := s.store.SaveMeta(meta); err != nil {
			return SessionMeta{}, err
		}
	}
	return meta, nil
}
