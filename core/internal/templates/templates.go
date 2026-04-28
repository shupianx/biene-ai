// Package templates owns the canonical list of model provider presets.
//
// This used to live in the renderer (renderer/src/constants/providerTemplates.ts)
// but moved to core for two reasons:
//
//  1. Templates are *data* (vendor URLs, model strings, context windows),
//     not *UI*. Splitting them put the source of truth in the wrong layer.
//  2. The config-migration pass needs to look up "what's the context
//     window for this saved model entry?". Co-locating that lookup with
//     the data eliminates the TS↔Go drift risk an alternative split would
//     have introduced.
//
// Icons and the "which template is the welcome modal's default cursor"
// are still owned by the renderer — those are UI concerns.
//
// To add a new model:
//   - Add its entry under the appropriate Vendor in `Builtin`.
//   - The renderer will pick it up at next /api/provider-templates fetch.
//   - The next launch's config v3 migration will backfill context_window
//     for any pre-existing config row that matches by (provider, model,
//     base_url).
package templates

// ProviderTemplate is a single model preset under a Vendor.
//
// JSON tags mirror what the renderer expects so the existing TS shape
// (renderer/src/api/http.ts ConfigModelEntry-like) keeps working when
// the templates fetch lands.
type ProviderTemplate struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Model             string         `json:"model"`
	ContextWindow     int            `json:"context_window,omitempty"`
	ThinkingAvailable bool           `json:"thinking_available,omitempty"`
	ThinkingOn        map[string]any `json:"thinking_on,omitempty"`
	ThinkingOff       map[string]any `json:"thinking_off,omitempty"`
	// ImagesAvailable mirrors the renderer's tri-state: nil = unspecified
	// (treated as true), false = explicitly vision-incapable.
	ImagesAvailable *bool `json:"images_available,omitempty"`
}

// Vendor groups several ProviderTemplate under a shared transport
// configuration (provider type + base URL + display name).
type Vendor struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Provider string             `json:"provider"`
	BaseURL  string             `json:"base_url"`
	Models   []ProviderTemplate `json:"models"`
}

func boolPtr(v bool) *bool { return &v }

// Builtin is the canonical list. Mirrors what used to live under
// renderer/src/constants/providerTemplates.ts. Treat this slice as the
// project's contract; the renderer fetches it via /api/provider-templates
// and renders icons + UI defaults on top of the data shape returned here.
var Builtin = []Vendor{
	{
		ID:       "anthropic",
		Name:     "Anthropic",
		Provider: "anthropic",
		BaseURL:  "https://api.anthropic.com",
		Models: []ProviderTemplate{
			{
				ID:                "claude-opus-4-7",
				Name:              "Opus 4.7",
				Model:             "claude-opus-4-7",
				ThinkingAvailable: true,
				ThinkingOn:        map[string]any{"thinking": map[string]any{"type": "enabled", "budget_tokens": 8000}},
				ThinkingOff:       map[string]any{"thinking": map[string]any{"type": "disabled"}},
				ContextWindow:     200000,
			},
			{
				ID:                "claude-sonnet-4-6",
				Name:              "Sonnet 4.6",
				Model:             "claude-sonnet-4-6",
				ThinkingAvailable: true,
				ThinkingOn:        map[string]any{"thinking": map[string]any{"type": "enabled", "budget_tokens": 8000}},
				ThinkingOff:       map[string]any{"thinking": map[string]any{"type": "disabled"}},
				ContextWindow:     200000,
			},
		},
	},
	{
		ID:       "openai",
		Name:     "OpenAI",
		Provider: "openai_compatible",
		BaseURL:  "https://api.openai.com/v1",
		Models: []ProviderTemplate{
			{ID: "gpt-5-5", Name: "GPT-5.5", Model: "gpt-5.5", ContextWindow: 200000},
			{ID: "gpt-5-4", Name: "GPT-5.4", Model: "gpt-5.4", ContextWindow: 200000},
		},
	},
	{
		ID:       "deepseek",
		Name:     "DeepSeek",
		Provider: "openai_compatible",
		BaseURL:  "https://api.deepseek.com",
		Models: []ProviderTemplate{
			{
				ID:                "deepseek-v4-pro",
				Name:              "V4-Pro",
				Model:             "deepseek-v4-pro",
				ThinkingAvailable: true,
				ThinkingOn:        map[string]any{"thinking": map[string]any{"type": "enabled"}},
				ThinkingOff:       map[string]any{"thinking": map[string]any{"type": "disabled"}},
				ImagesAvailable:   boolPtr(false),
				ContextWindow:     128000,
			},
			{
				ID:                "deepseek-v4-flash",
				Name:              "V4-Flash",
				Model:             "deepseek-v4-flash",
				ThinkingAvailable: true,
				ThinkingOn:        map[string]any{"thinking": map[string]any{"type": "enabled"}},
				ThinkingOff:       map[string]any{"thinking": map[string]any{"type": "disabled"}},
				ImagesAvailable:   boolPtr(false),
				ContextWindow:     128000,
			},
			{
				ID:                "deepseek-v3-2",
				Name:              "V3.2 (chat)",
				Model:             "deepseek-chat",
				ThinkingAvailable: true,
				ThinkingOn:        map[string]any{"model": "deepseek-reasoner"},
				ContextWindow:     128000,
			},
		},
	},
	{
		ID:       "qwen",
		Name:     "Qwen",
		Provider: "openai_compatible",
		BaseURL:  "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Models: []ProviderTemplate{
			{
				ID:                "qwen3-6-plus",
				Name:              "Qwen3.6-plus",
				Model:             "qwen3.6-plus",
				ThinkingAvailable: true,
				ThinkingOn:        map[string]any{"enable_thinking": true},
				ThinkingOff:       map[string]any{"enable_thinking": false},
				ContextWindow:     131072,
			},
		},
	},
	{
		ID:       "kimi",
		Name:     "Kimi (Moonshot)",
		Provider: "openai_compatible",
		BaseURL:  "https://api.moonshot.cn/v1",
		Models: []ProviderTemplate{
			{
				ID:                "kimi-k2-6",
				Name:              "K2.6",
				Model:             "kimi-k2.6",
				ThinkingAvailable: true,
				ThinkingOn:        map[string]any{"thinking": map[string]any{"type": "enabled"}},
				ThinkingOff:       map[string]any{"thinking": map[string]any{"type": "disabled"}},
				ContextWindow:     200000,
			},
		},
	},
}
