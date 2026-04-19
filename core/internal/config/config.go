package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const configFileName = ".biene/config.json"

// ModelEntry holds one selectable model configuration.
type ModelEntry struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Provider       string `json:"provider"` // "anthropic" | "openai_compatible"
	APIKey         string `json:"api_key"`
	Model          string `json:"model"`
	BaseURL        string `json:"base_url"`
	EnableThinking bool   `json:"enable_thinking,omitempty"`
}

// Settings holds global behavior settings.
type Settings struct {
	MaxTokens int `json:"max_tokens"`
}

// Config is the root configuration structure.
type Config struct {
	DefaultModel string       `json:"default_model"`
	ModelList    []ModelEntry `json:"model_list"`
	Settings     Settings     `json:"settings"`
}

// LoadResult carries the loaded config plus metadata about how it was loaded.
type LoadResult struct {
	Config  *Config
	Path    string
	Created bool
}

// TemplateConfig returns an empty config template for first-time setup.
func TemplateConfig() *Config {
	return &Config{
		DefaultModel: "main",
		ModelList: []ModelEntry{
			{
				ID:       "main",
				Name:     "main",
				Provider: "anthropic",
				APIKey:   "",
				Model:    "claude-opus-4-6",
				BaseURL:  "",
			},
		},
		Settings: Settings{
			MaxTokens: 8192,
		},
	}
}

// configPath returns the path to the config file.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, configFileName), nil
}

// Path returns the path to ~/.biene/config.json.
func Path() (string, error) {
	return configPath()
}

// Load reads the config file from ~/.biene/config.json.
// If the file does not exist, an empty template config is written and returned.
func Load() (*LoadResult, error) {
	path, err := configPath()
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
	needsSave := Normalize(&cfg)
	if needsSave {
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
	Normalize(cfg)
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing config: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// Normalize fills defaults, migrates legacy name-based configs to id-based
// configs, and rewrites provider/id fields into a stable shape.
func Normalize(cfg *Config) bool {
	if cfg == nil {
		return false
	}

	changed := false
	if len(cfg.ModelList) == 0 {
		cfg.ModelList = []ModelEntry{defaultModelEntry()}
		changed = true
	}

	legacyNameToID := make(map[string]string, len(cfg.ModelList))
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

		if shouldAutoEnableThinking(*entry) && !entry.EnableThinking {
			entry.EnableThinking = true
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
		legacyNameToID[entry.Name] = entry.ID
	}

	defaultModel := strings.TrimSpace(cfg.DefaultModel)
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
	case legacyNameToID[defaultModel] != "":
		cfg.DefaultModel = legacyNameToID[defaultModel]
		changed = true
	default:
		cfg.DefaultModel = cfg.ModelList[0].ID
		changed = true
	}

	if cfg.Settings.MaxTokens == 0 {
		cfg.Settings.MaxTokens = 8192
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

func shouldAutoEnableThinking(entry ModelEntry) bool {
	if normalizeProvider(entry.Provider) != "openai_compatible" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(entry.Model), "qwen3.6-plus")
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
