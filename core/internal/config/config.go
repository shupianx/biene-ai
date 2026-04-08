package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const configFileName = ".biene/config.json"

// ModelEntry holds one selectable model configuration.
type ModelEntry struct {
	Name     string `json:"name"`
	Provider string `json:"provider"` // "anthropic" | "openai_compatible"
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url"`
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
	needsSave := false
	if cfg.DefaultModel == "" && len(cfg.ModelList) > 0 {
		cfg.DefaultModel = cfg.ModelList[0].Name
		needsSave = true
	}
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = "main"
		needsSave = true
	}
	if len(cfg.ModelList) == 0 {
		cfg.ModelList = []ModelEntry{{
			Name:     "main",
			Provider: "anthropic",
			APIKey:   "",
			Model:    "claude-opus-4-6",
			BaseURL:  "",
		}}
		needsSave = true
	}
	for i := range cfg.ModelList {
		if cfg.ModelList[i].Name == "" {
			cfg.ModelList[i].Name = "main"
			needsSave = true
		}
		if cfg.ModelList[i].Provider == "" {
			cfg.ModelList[i].Provider = "anthropic"
			needsSave = true
		}
		if cfg.ModelList[i].Model == "" {
			cfg.ModelList[i].Model = "claude-opus-4-6"
			needsSave = true
		}
	}
	if cfg.Settings.MaxTokens == 0 {
		cfg.Settings.MaxTokens = 8192
		needsSave = true
	}
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

// GetModel returns the named model entry, falling back to the default model.
func (c *Config) GetModel(name string) (ModelEntry, error) {
	if name == "" {
		name = c.DefaultModel
	}
	if name == "" && len(c.ModelList) > 0 {
		return c.ModelList[0], nil
	}
	for _, entry := range c.ModelList {
		if entry.Name == name {
			return entry, nil
		}
	}
	return ModelEntry{}, fmt.Errorf("model %q not found in config", name)
}
