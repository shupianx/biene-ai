package skills

import (
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"biene/internal/bienehome"
)

type skillRepositoryConfig struct {
	DefaultEnabledSkillDirs []string `json:"defaultEnabledSkillDirs"`
	DefaultSkillDir         string   `json:"defaultSkillDir,omitempty"`
}

func loadSkillRepositoryConfig() (skillRepositoryConfig, error) {
	path, err := bienehome.SkillConfigPath()
	if err != nil {
		return skillRepositoryConfig{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return skillRepositoryConfig{}, nil
		}
		return skillRepositoryConfig{}, err
	}

	var cfg skillRepositoryConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return skillRepositoryConfig{}, err
	}

	cfg.DefaultEnabledSkillDirs = normalizeDefaultEnabledSkillDirs(cfg.DefaultEnabledSkillDirs, cfg.DefaultSkillDir)
	return cfg, nil
}

func normalizeDefaultEnabledSkillDirs(items []string, legacy string) []string {
	seen := make(map[string]struct{}, len(items)+1)
	out := make([]string, 0, len(items)+1)

	add := func(value string) {
		value = filepath.Clean(value)
		if value == "." || value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	for _, item := range items {
		add(item)
	}
	add(legacy)
	return out
}

// InstallDefaultEnabled copies repository-configured default-enabled skills
// into a new agent workspace under <workDir>/.biene/skills.
func InstallDefaultEnabled(workDir string) error {
	cfg, err := loadSkillRepositoryConfig()
	if err != nil {
		return err
	}
	if len(cfg.DefaultEnabledSkillDirs) == 0 {
		return nil
	}

	root, err := EnsureRepositoryRoot()
	if err != nil {
		return err
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	destRoot := filepath.Join(workDir, bienehome.DirName, "skills")
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return err
	}

	for _, configuredDir := range cfg.DefaultEnabledSkillDirs {
		srcReal, ok, err := resolveConfiguredSkillDir(rootReal, configuredDir)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		rel, err := filepath.Rel(rootReal, srcReal)
		if err != nil {
			return err
		}
		destDir := filepath.Join(destRoot, rel)
		if err := copyDir(srcReal, destDir); err != nil {
			return err
		}
	}

	return nil
}

func resolveConfiguredSkillDir(repositoryRootReal, configured string) (string, bool, error) {
	if configured == "" {
		return "", false, nil
	}

	srcReal, err := filepath.EvalSymlinks(configured)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}

	if srcReal == repositoryRootReal || !hasPathPrefix(srcReal, repositoryRootReal) {
		return "", false, nil
	}

	info, err := os.Stat(srcReal)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if !info.IsDir() {
		return "", false, nil
	}
	if _, err := os.Stat(filepath.Join(srcReal, skillFileName)); err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}

	return srcReal, true, nil
}

func hasPathPrefix(target, root string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." || rel == "" {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
