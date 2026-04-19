package bienehome

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DirName             = ".biene"
	configFileName      = "config.json"
	skillConfigFileName = "skill-config.json"
	skillsDirName       = "skills"
)

// HomeDir returns the global ~/.biene directory.
func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, DirName), nil
}

// ConfigPath returns the path to ~/.biene/config.json.
func ConfigPath() (string, error) {
	return pathFor(configFileName)
}

// SkillConfigPath returns the path to ~/.biene/skill-config.json.
func SkillConfigPath() (string, error) {
	return pathFor(skillConfigFileName)
}

// SkillRepositoryRoot returns the path to ~/.biene/skills.
func SkillRepositoryRoot() (string, error) {
	return pathFor(skillsDirName)
}

// EnsureSkillRepositoryRoot creates ~/.biene/skills when it does not exist.
func EnsureSkillRepositoryRoot() (string, error) {
	root, err := SkillRepositoryRoot()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	return root, nil
}

// WriteJSON writes a JSON file with consistent formatting and a trailing newline.
func WriteJSON(path string, v any, dirMode, fileMode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return err
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), fileMode)
}

func pathFor(name string) (string, error) {
	root, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}
