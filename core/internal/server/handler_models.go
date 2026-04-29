package server

import (
	"net/http"
	"strings"

	"biene/internal/auth"
)

// formatChatGPTModelLabel turns a raw OpenAI model id ("gpt-5.5") into
// the friendlier form shown in the New Agent dropdown ("GPT-5.5"). The
// rest of the id stays untouched so future model names with mixed
// segments (e.g. "gpt-5.5-codex") still read sensibly.
func formatChatGPTModelLabel(model string) string {
	if strings.HasPrefix(model, "gpt-") {
		return "GPT-" + model[len("gpt-"):]
	}
	return strings.ToUpper(model)
}

// availableModelGroup groups model options under a single header in the
// New Agent dropdown. The renderer is responsible for visual hierarchy;
// this endpoint just declares "these belong together, this one is a
// virtual provider that needs sub-selection".
type availableModelGroup struct {
	ID          string                `json:"id"`
	Label       string                `json:"label"`
	Kind        string                `json:"kind"` // "user" | "chatgpt_official"
	Models      []availableModelEntry `json:"models"`
	Description string                `json:"description,omitempty"`
}

// availableModelEntry is a single picker entry. For user configs the
// id is the canonical NamedModelConfig id; for the synthetic OAuth
// provider it is "chatgpt_official:<model>".
type availableModelEntry struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Summary  string `json:"summary,omitempty"`
}

// availableModelsResponse is the wire shape consumed by NewAgentModal.
// DefaultModelID points into a user-defined config — the OAuth synthetic
// entries are never the default, so existing default-selection behavior
// stays unchanged.
type availableModelsResponse struct {
	DefaultModelID string                `json:"default_model_id"`
	Groups         []availableModelGroup `json:"groups"`
}

// handleAvailableModels returns the unified picker list: user configs
// from ~/.biene/config.json plus, when the user is signed in, a single
// "ChatGPT (official)" group whose entries route through the OAuth
// provider wrapper.
//
// This is intentionally separate from GET /api/config: that endpoint
// is the editing surface (only user-managed entries), this one is the
// runtime "what can I attach to a session" surface (also includes
// auth-gated synthetic providers).
func (s *Server) handleAvailableModels(w http.ResponseWriter, _ *http.Request) {
	resp := availableModelsResponse{
		DefaultModelID: s.cfg.DefaultModel,
		Groups:         []availableModelGroup{},
	}

	if len(s.cfg.ModelList) > 0 {
		userGroup := availableModelGroup{
			ID:    "user",
			Label: "user",
			Kind:  "user",
		}
		for _, entry := range s.cfg.ModelList {
			userGroup.Models = append(userGroup.Models, availableModelEntry{
				ID:       entry.ID,
				Label:    entry.Name,
				Provider: entry.Provider,
				Model:    entry.Model,
				Summary:  entry.Model,
			})
		}
		resp.Groups = append(resp.Groups, userGroup)
	}

	if s.chatgptAuth != nil && s.chatgptAuth.Authenticated() {
		group := availableModelGroup{
			ID:          "chatgpt_official",
			Label:       "ChatGPT (official)",
			Kind:        "chatgpt_official",
			Description: "Signed in via ChatGPT OAuth",
		}
		for _, model := range auth.ChatGPTOfficialModels() {
			group.Models = append(group.Models, availableModelEntry{
				ID:       "chatgpt_official:" + model,
				Label:    formatChatGPTModelLabel(model),
				Provider: "chatgpt_official",
				Model:    model,
				Summary:  model,
			})
		}
		resp.Groups = append(resp.Groups, group)
	}

	writeJSON(w, http.StatusOK, resp)
}
