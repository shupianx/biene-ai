package server

import (
	"net/http"
)

// publicModelEntry is a sanitised view of config.ModelEntry (no api_key).
type publicModelEntry struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url"`
}

// publicConfig is the response body for GET /api/config.
type publicConfig struct {
	DefaultModel string             `json:"default_model"`
	ModelList    []publicModelEntry `json:"model_list"`
	MaxTokens    int                `json:"max_tokens"`
}

// handleConfig returns the current configuration (API keys stripped).
// GET /api/config
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg

	entries := make([]publicModelEntry, len(cfg.ModelList))
	for i, e := range cfg.ModelList {
		entries[i] = publicModelEntry{
			Name:     e.Name,
			Provider: e.Provider,
			Model:    e.Model,
			BaseURL:  e.BaseURL,
		}
	}

	resp := publicConfig{
		DefaultModel: cfg.DefaultModel,
		ModelList:    entries,
		MaxTokens:    cfg.Settings.MaxTokens,
	}

	writeJSON(w, http.StatusOK, resp)
}
