package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"biene/internal/config"
)

type editableModelEntry struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Provider          string         `json:"provider"`
	APIKey            string         `json:"api_key"`
	Model             string         `json:"model"`
	BaseURL           string         `json:"base_url"`
	ThinkingAvailable bool           `json:"thinking_available,omitempty"`
	ThinkingOn        map[string]any `json:"thinking_on,omitempty"`
	ThinkingOff       map[string]any `json:"thinking_off,omitempty"`
	ImagesAvailable   *bool          `json:"images_available,omitempty"`
}

type editableConfig struct {
	DefaultModel string               `json:"default_model"`
	ModelList    []editableModelEntry `json:"model_list"`
}

// handleConfig returns the current configuration.
// GET /api/config
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, configResponse(s.cfg))
}

// handleUpdateConfig saves the current configuration and applies it in memory.
// POST /api/config
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req editableConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateEditableConfig(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	cfg := &config.Config{
		DefaultModel: req.DefaultModel,
		ModelList:    make([]config.ModelEntry, len(req.ModelList)),
	}
	for i, entry := range req.ModelList {
		cfg.ModelList[i] = config.ModelEntry{
			ID:                entry.ID,
			Name:              entry.Name,
			Provider:          entry.Provider,
			APIKey:            entry.APIKey,
			Model:             entry.Model,
			BaseURL:           entry.BaseURL,
			ThinkingAvailable: entry.ThinkingAvailable,
			ThinkingOn:        entry.ThinkingOn,
			ThinkingOff:       entry.ThinkingOff,
			ImagesAvailable:   entry.ImagesAvailable,
		}
	}

	config.Normalize(cfg)
	if len(cfg.ModelList) == 0 {
		writeError(w, http.StatusBadRequest, "at least one model provider is required")
		return
	}
	if inUse := firstRemovedModelInUse(s.cfg, cfg, s.mgr.ModelUsageCounts()); inUse != "" {
		writeError(w, http.StatusBadRequest, "cannot delete a model provider that is still used by an agent")
		return
	}
	if _, err := cfg.GetModel(""); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := config.Save(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.mgr.UpdateConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.cfg = cfg
	writeJSON(w, http.StatusOK, configResponse(cfg))
}

func validateEditableConfig(req editableConfig) error {
	if len(req.ModelList) == 0 {
		return httpError("at least one model provider is required")
	}

	seen := make(map[string]struct{}, len(req.ModelList))
	for _, entry := range req.ModelList {
		id := config.NormalizeModelID(entry.ID)
		if id == "" {
			return httpError("provider id is required")
		}
		if _, exists := seen[id]; exists {
			return httpError("provider id already exists")
		}
		seen[id] = struct{}{}

		if strings.TrimSpace(entry.Name) == "" {
			return httpError("provider name is required")
		}
		if strings.TrimSpace(entry.Model) == "" {
			return httpError("provider model is required")
		}
	}

	defaultModel := strings.TrimSpace(req.DefaultModel)
	if defaultModel == "" {
		return nil
	}
	defaultModel = config.NormalizeModelID(defaultModel)
	if _, exists := seen[defaultModel]; exists {
		return nil
	}
	return httpError("default model not found")
}

type httpError string

func (e httpError) Error() string { return string(e) }

func configResponse(cfg *config.Config) editableConfig {
	entries := make([]editableModelEntry, len(cfg.ModelList))
	for i, e := range cfg.ModelList {
		entries[i] = editableModelEntry{
			ID:                e.ID,
			Name:              e.Name,
			Provider:          e.Provider,
			APIKey:            e.APIKey,
			Model:             e.Model,
			BaseURL:           e.BaseURL,
			ThinkingAvailable: e.ThinkingAvailable,
			ThinkingOn:        e.ThinkingOn,
			ThinkingOff:       e.ThinkingOff,
			ImagesAvailable:   e.ImagesAvailable,
		}
	}

	return editableConfig{
		DefaultModel: cfg.DefaultModel,
		ModelList:    entries,
	}
}

func firstRemovedModelInUse(prev, next *config.Config, usage map[string]int) string {
	if prev == nil {
		return ""
	}

	nextIDs := make(map[string]struct{}, len(next.ModelList))
	for _, entry := range next.ModelList {
		nextIDs[entry.ID] = struct{}{}
	}

	for _, entry := range prev.ModelList {
		if _, exists := nextIDs[entry.ID]; exists {
			continue
		}
		if usage[entry.ID] > 0 {
			return entry.ID
		}
	}
	return ""
}
