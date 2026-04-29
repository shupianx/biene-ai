package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"biene/internal/bienehome"
)

// ModelEntry holds one selectable model configuration.
//
// ThinkingAvailable gates whether the session-level thinking toggle is shown
// in the UI. ThinkingOn / ThinkingOff are provider-specific JSON fragments
// that are shallow-merged into the chat request body when thinking is enabled
// or disabled respectively. Providers that express thinking differently
// (Qwen top-level `enable_thinking`, Kimi nested `thinking.type`, etc.)
// carry their own shapes here so the core stays provider-agnostic.
type ModelEntry struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	Provider          string         `json:"provider"` // "anthropic" | "openai_compatible"
	APIKey            string         `json:"api_key"`
	Model             string         `json:"model"`
	BaseURL           string         `json:"base_url"`
	ThinkingAvailable bool           `json:"thinking_available,omitempty"`
	ThinkingOn        map[string]any `json:"thinking_on,omitempty"`
	ThinkingOff       map[string]any `json:"thinking_off,omitempty"`
	// ImagesAvailable advertises whether the model accepts image inputs.
	// `nil` means "unspecified" and is treated as true at every read site
	// so existing entries (and hand-edited config files that omit the
	// field) keep working. Only an explicit `false` makes the renderer
	// hide its image attach control.
	ImagesAvailable *bool `json:"images_available,omitempty"`
	// ContextWindow caps the model's combined input+output token capacity.
	// Compaction triggers when input usage gets within `reserve_tokens` of
	// this value. 0 means "unset" — falls back to DefaultContextWindow.
	ContextWindow int `json:"context_window,omitempty"`
	// ServiceTier is the OpenAI Codex `service_tier` knob (empty =
	// upstream default). Recognised values are "default" | "flex" |
	// "priority" | "scale" | "auto". Only the chatgpt_official provider
	// currently consumes it — every other provider ignores it. Set on
	// the synthetic ChatGPT entry via env var BIENE_CHATGPT_SERVICE_TIER
	// or hand-edit the relevant ModelEntry.
	ServiceTier string `json:"service_tier,omitempty"`
}

// CompactionConfig controls automatic context compression.
//
// Disabled compaction lets the conversation grow until the API rejects a
// request; users can still trigger /compact manually.
type CompactionConfig struct {
	Enabled          bool `json:"enabled"`
	ReserveTokens    int  `json:"reserve_tokens"`     // headroom kept free for the model's response
	KeepRecentTokens int  `json:"keep_recent_tokens"` // tail-of-history token budget preserved verbatim
}

// DefaultContextWindow is the conservative fallback applied when a model
// entry leaves ContextWindow at 0. 32K covers nearly every commonly hosted
// open-weights model; users with larger windows fill in the explicit value.
const DefaultContextWindow = 32000

// Default compaction tuning — see compaction discussion in repo notes.
// reserveTokens = 20000 (headroom for output + tool overhead),
// keepRecentTokens = 32000 (preserve last few turns including tool results).
const (
	DefaultCompactionReserve    = 20000
	DefaultCompactionKeepRecent = 32000
)

// DefaultCompactionConfig returns the built-in compaction tuning. Used when
// the loaded config has no `compaction` block (legacy v1 files).
func DefaultCompactionConfig() CompactionConfig {
	return CompactionConfig{
		Enabled:          true,
		ReserveTokens:    DefaultCompactionReserve,
		KeepRecentTokens: DefaultCompactionKeepRecent,
	}
}

// Config is the root configuration structure.
//
// `Version` tracks the schema version this file was written under. When
// Load() reads a file with a lower version, the migrations in
// `configMigrations` are applied in order to bring it up to
// `CurrentConfigVersion`. A missing field decodes as 0, which matches the
// pre-versioning baseline.
type Config struct {
	Version      int               `json:"version,omitempty"`
	DefaultModel string            `json:"default_model"`
	ModelList    []ModelEntry      `json:"model_list"`
	Compaction   *CompactionConfig `json:"compaction,omitempty"`
}

// CompactionSettings returns the active compaction tuning, falling back to
// built-in defaults when the field is absent in the loaded config.
func (c *Config) CompactionSettings() CompactionConfig {
	if c == nil || c.Compaction == nil {
		return DefaultCompactionConfig()
	}
	return *c.Compaction
}

// LoadResult carries the loaded config plus metadata about how it was loaded.
type LoadResult struct {
	Config  *Config
	Path    string
	Created bool
}

// TemplateConfig returns an empty config template for first-time setup.
// ModelList is intentionally empty: the onboarding flow asks the user to
// configure their first provider before any agent can be created.
func TemplateConfig() *Config {
	def := DefaultCompactionConfig()
	return &Config{
		Version:      CurrentConfigVersion,
		DefaultModel: "",
		ModelList:    []ModelEntry{},
		Compaction:   &def,
	}
}

// Path returns the path to ~/.biene/config.json.
func Path() (string, error) {
	return bienehome.ConfigPath()
}

// Load reads the config file from ~/.biene/config.json.
// If the file does not exist, an empty template config is written and returned.
func Load() (*LoadResult, error) {
	path, err := bienehome.ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := TemplateConfig()
		if saveErr := Save(cfg); saveErr != nil {
			return nil, fmt.Errorf("creating config template: %w", saveErr)
		}
		return &LoadResult{
			Config:  cfg,
			Path:    path,
			Created: true,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	// Run schema migrations before Normalize so later steps see the
	// upgraded shape. Migrate + Normalize are both idempotent, so calling
	// them on every Load is safe; the boolean reports whether anything
	// changed and only then do we rewrite the file.
	migrated := Migrate(&cfg)
	normalized := Normalize(&cfg)
	if migrated || normalized {
		if err := Save(&cfg); err != nil {
			return nil, fmt.Errorf("updating config file: %w", err)
		}
	}
	return &LoadResult{
		Config: &cfg,
		Path:   path,
	}, nil
}

// Save writes the config to ~/.biene/config.json, creating directories as needed.
func Save(cfg *Config) error {
	Migrate(cfg)
	Normalize(cfg)
	path, err := bienehome.ConfigPath()
	if err != nil {
		return err
	}
	if err := bienehome.WriteJSON(path, cfg, 0o700, 0o600); err != nil {
		return fmt.Errorf("saving config file: %w", err)
	}
	return nil
}

// Normalize fills defaults and rewrites provider/id fields into a stable shape.
func Normalize(cfg *Config) bool {
	if cfg == nil {
		return false
	}

	changed := false
	if cfg.ModelList == nil {
		cfg.ModelList = []ModelEntry{}
	}

	usedIDs := make(map[string]struct{}, len(cfg.ModelList))
	for i := range cfg.ModelList {
		entry := &cfg.ModelList[i]

		name := strings.TrimSpace(entry.Name)
		if name == "" {
			switch {
			case strings.TrimSpace(entry.ID) != "":
				name = strings.TrimSpace(entry.ID)
			case i == 0:
				name = "main"
			default:
				name = fmt.Sprintf("model-%d", i+1)
			}
		}
		if name != entry.Name {
			entry.Name = name
			changed = true
		}

		provider := normalizeProvider(entry.Provider)
		if provider != entry.Provider {
			entry.Provider = provider
			changed = true
		}

		apiKey := strings.TrimSpace(entry.APIKey)
		if apiKey != entry.APIKey {
			entry.APIKey = apiKey
			changed = true
		}

		model := strings.TrimSpace(entry.Model)
		if model == "" {
			model = defaultModelEntry().Model
		}
		if model != entry.Model {
			entry.Model = model
			changed = true
		}

		baseURL := strings.TrimSpace(entry.BaseURL)
		if baseURL != entry.BaseURL {
			entry.BaseURL = baseURL
			changed = true
		}

		// ContextWindow < 0 is nonsense; keep 0 as "unset" so the runtime
		// fallback to DefaultContextWindow stays observable.
		if entry.ContextWindow < 0 {
			entry.ContextWindow = 0
			changed = true
		}

		requestedID := strings.TrimSpace(entry.ID)
		if requestedID == "" {
			requestedID = entry.Name
		}
		id := uniqueModelID(sanitizeModelID(requestedID), usedIDs)
		if id == "" {
			id = uniqueModelID(fmt.Sprintf("model-%d", i+1), usedIDs)
		}
		if id != entry.ID {
			entry.ID = id
			changed = true
		}
	}

	defaultModel := strings.TrimSpace(cfg.DefaultModel)
	if len(cfg.ModelList) == 0 {
		if cfg.DefaultModel != "" {
			cfg.DefaultModel = ""
			changed = true
		}
		return changed
	}

	switch {
	case defaultModel == "":
		cfg.DefaultModel = cfg.ModelList[0].ID
		changed = true
	case hasModelID(cfg.ModelList, defaultModel):
		if cfg.DefaultModel != defaultModel {
			cfg.DefaultModel = defaultModel
			changed = true
		}
	case hasModelID(cfg.ModelList, sanitizeModelID(defaultModel)):
		cfg.DefaultModel = sanitizeModelID(defaultModel)
		changed = true
	default:
		cfg.DefaultModel = cfg.ModelList[0].ID
		changed = true
	}

	if normalizeCompaction(cfg) {
		changed = true
	}

	return changed
}

// normalizeCompaction clamps obviously broken values and fills the block
// with defaults when absent. Returns true if anything changed.
func normalizeCompaction(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.Compaction == nil {
		def := DefaultCompactionConfig()
		cfg.Compaction = &def
		return true
	}
	changed := false
	if cfg.Compaction.ReserveTokens <= 0 {
		cfg.Compaction.ReserveTokens = DefaultCompactionReserve
		changed = true
	}
	if cfg.Compaction.KeepRecentTokens <= 0 {
		cfg.Compaction.KeepRecentTokens = DefaultCompactionKeepRecent
		changed = true
	}
	return changed
}

// GetModel returns the named model entry by id, falling back to the default model id.
func (c *Config) GetModel(id string) (ModelEntry, error) {
	if id == "" {
		id = c.DefaultModel
	}
	if id == "" && len(c.ModelList) > 0 {
		return c.ModelList[0], nil
	}
	for _, entry := range c.ModelList {
		if entry.ID == id {
			return entry, nil
		}
	}
	return ModelEntry{}, fmt.Errorf("model %q not found in config", id)
}

func defaultModelEntry() ModelEntry {
	return ModelEntry{
		ID:       "main",
		Name:     "main",
		Provider: "anthropic",
		APIKey:   "",
		Model:    "claude-opus-4-6",
		BaseURL:  "",
	}
}

func normalizeProvider(provider string) string {
	switch strings.TrimSpace(provider) {
	case "openai", "openai_compatible":
		return "openai_compatible"
	default:
		return "anthropic"
	}
}

// NormalizeModelID rewrites a model/provider id into the persisted config form.
func NormalizeModelID(value string) string {
	return sanitizeModelID(value)
}

func sanitizeModelID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	var sb strings.Builder
	lastWasDash := false
	for _, r := range value {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		switch {
		case isAlphaNum:
			sb.WriteRune(r)
			lastWasDash = false
		case r == '_' || r == '-':
			if sb.Len() == 0 || lastWasDash {
				continue
			}
			sb.WriteByte('-')
			lastWasDash = true
		default:
			if sb.Len() == 0 || lastWasDash {
				continue
			}
			sb.WriteByte('-')
			lastWasDash = true
		}
	}

	return strings.Trim(sb.String(), "-")
}

func uniqueModelID(base string, used map[string]struct{}) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "model"
	}
	if _, exists := used[base]; !exists {
		used[base] = struct{}{}
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, exists := used[candidate]; exists {
			continue
		}
		used[candidate] = struct{}{}
		return candidate
	}
}

func hasModelID(entries []ModelEntry, id string) bool {
	for _, entry := range entries {
		if entry.ID == id {
			return true
		}
	}
	return false
}
