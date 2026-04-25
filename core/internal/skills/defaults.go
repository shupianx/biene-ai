package skills

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"biene/internal/bienehome"
)

type skillRepositoryConfig struct {
	DefaultEnabledSkillDirs []string `json:"defaultEnabledSkillDirs"`
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

	cfg.DefaultEnabledSkillDirs = normalizeDefaultEnabledSkillDirs(cfg.DefaultEnabledSkillDirs)
	return cfg, nil
}

func normalizeDefaultEnabledSkillDirs(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))

	for _, item := range items {
		value := filepath.Clean(item)
		if value == "." || value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// InstallSkillByID copies one repository skill identified by its stable ID
// into the agent workspace at <workDir>/.biene/skills/<rel>. If a directory
// already exists at the destination, it is removed first so files deleted
// from the repository version do not linger. Returns the skill's canonical
// Name (from its frontmatter) on success. Callers that want to warn the user
// before overwriting should check for an existing install via
// InstalledSkillIDsForWorkDir before calling this function.
func InstallSkillByID(workDir, id string) (string, error) {
	id = strings.TrimSpace(filepath.ToSlash(id))
	if id == "" {
		return "", fmt.Errorf("skill id is required")
	}

	metas, root, err := ScanRepository()
	if err != nil {
		return "", err
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}

	var srcMeta *Metadata
	for i := range metas {
		if RepositorySkillID(root, metas[i].Dir) == id {
			srcMeta = &metas[i]
			break
		}
	}
	if srcMeta == nil {
		return "", fmt.Errorf("skill not found: %s", id)
	}

	srcReal, err := filepath.EvalSymlinks(srcMeta.Dir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootReal, srcReal)
	if err != nil {
		return "", err
	}

	destRoot := filepath.Join(workDir, bienehome.DirName, "skills")
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return "", err
	}
	destDir := filepath.Join(destRoot, rel)

	if err := os.RemoveAll(destDir); err != nil {
		return "", err
	}
	if err := copyDir(srcReal, destDir); err != nil {
		return "", err
	}

	return srcMeta.Name, nil
}

// UninstallSkillByID removes one installed skill from the agent workspace.
// Returns the skill's canonical Name (from its frontmatter) when the skill
// was present and removed, or an empty name when the target did not exist.
// The id is the same stable ID produced by InstalledSkillIDsForWorkDir.
func UninstallSkillByID(workDir, id string) (string, error) {
	id = strings.TrimSpace(filepath.ToSlash(id))
	if id == "" {
		return "", fmt.Errorf("skill id is required")
	}

	root := WorkDirSkillsRoot(workDir)
	metas, err := ScanFromDir(root)
	if err != nil {
		return "", err
	}

	var targetMeta *Metadata
	for i := range metas {
		if RepositorySkillID(root, metas[i].Dir) == id {
			targetMeta = &metas[i]
			break
		}
	}
	if targetMeta == nil {
		return "", fmt.Errorf("skill not found: %s", id)
	}

	if err := os.RemoveAll(targetMeta.Dir); err != nil {
		return "", err
	}
	return targetMeta.Name, nil
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
