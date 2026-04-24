package skills

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"tinte/internal/tintehome"
)

// RepositoryConfig stores skill repository preferences under ~/.tinte/skill-config.json.
type RepositoryConfig struct {
	DefaultEnabledSkillDirs []string `json:"defaultEnabledSkillDirs"`
}

// UploadedFile represents one uploaded file for a skill repository import.
type UploadedFile struct {
	Path string
	Open func() (io.ReadCloser, error)
}

// LoadRepositoryConfig returns the normalized skill repository config.
func LoadRepositoryConfig() (RepositoryConfig, error) {
	path, err := tintehome.SkillConfigPath()
	if err != nil {
		return RepositoryConfig{}, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return SaveRepositoryConfig(RepositoryConfig{})
	} else if err != nil {
		return RepositoryConfig{}, err
	}

	cfg, err := loadSkillRepositoryConfig()
	if err != nil {
		return RepositoryConfig{}, err
	}
	return RepositoryConfig{
		DefaultEnabledSkillDirs: append([]string(nil), cfg.DefaultEnabledSkillDirs...),
	}, nil
}

// SaveRepositoryConfig writes the normalized skill repository config.
func SaveRepositoryConfig(cfg RepositoryConfig) (RepositoryConfig, error) {
	normalized := RepositoryConfig{
		DefaultEnabledSkillDirs: normalizeDefaultEnabledSkillDirs(cfg.DefaultEnabledSkillDirs),
	}

	path, err := tintehome.SkillConfigPath()
	if err != nil {
		return RepositoryConfig{}, err
	}
	if err := tintehome.WriteJSON(path, normalized, 0o700, 0o600); err != nil {
		return RepositoryConfig{}, err
	}
	return normalized, nil
}

// RepositorySkillID returns the stable API identifier for a discovered repository skill.
func RepositorySkillID(root, dir string) string {
	root = filepath.Clean(root)
	dir = filepath.Clean(dir)

	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." || rel == "" {
		return ""
	}
	return filepath.ToSlash(rel)
}

// SetRepositoryDefaultEnabledByID rewrites the repository config using stable skill IDs.
func SetRepositoryDefaultEnabledByID(ids []string) (RepositoryConfig, error) {
	metas, root, err := ScanRepository()
	if err != nil {
		return RepositoryConfig{}, err
	}

	byID := make(map[string]string, len(metas))
	for _, meta := range metas {
		id := RepositorySkillID(root, meta.Dir)
		if id == "" {
			continue
		}
		byID[id] = meta.Dir
	}

	dirs := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(filepath.ToSlash(id))
		if id == "" {
			continue
		}
		dir, ok := byID[id]
		if !ok {
			return RepositoryConfig{}, fmt.Errorf("skill not found: %s", id)
		}
		dirs = append(dirs, dir)
	}

	return SaveRepositoryConfig(RepositoryConfig{DefaultEnabledSkillDirs: dirs})
}

// DeleteRepositorySkillByID removes one repository skill and updates the default-enabled list.
func DeleteRepositorySkillByID(id string) (RepositoryConfig, error) {
	metas, root, err := ScanRepository()
	if err != nil {
		return RepositoryConfig{}, err
	}

	var targetDir string
	for _, meta := range metas {
		if RepositorySkillID(root, meta.Dir) == id {
			targetDir = meta.Dir
			break
		}
	}
	if targetDir == "" {
		return RepositoryConfig{}, fmt.Errorf("skill not found: %s", id)
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return RepositoryConfig{}, err
	}

	cfg, err := LoadRepositoryConfig()
	if err != nil {
		return RepositoryConfig{}, err
	}
	cfg.DefaultEnabledSkillDirs = slices.DeleteFunc(cfg.DefaultEnabledSkillDirs, func(entry string) bool {
		return filepath.Clean(entry) == filepath.Clean(targetDir)
	})

	return SaveRepositoryConfig(cfg)
}

// ImportRepositoryFiles imports one uploaded skill directory into ~/.tinte/skills.
func ImportRepositoryFiles(files []UploadedFile) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no files uploaded")
	}

	rootName, err := uploadedRootName(files)
	if err != nil {
		return "", err
	}

	repositoryRoot, err := EnsureRepositoryRoot()
	if err != nil {
		return "", err
	}

	targetDir := uniqueImportDir(repositoryRoot, rootName)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}

	success := false
	defer func() {
		if success {
			return
		}
		_ = os.RemoveAll(targetDir)
	}()

	for _, file := range files {
		relPath, err := normalizeUploadedRelativePath(file.Path, rootName)
		if err != nil {
			return "", err
		}
		if relPath == "" {
			continue
		}

		src, err := file.Open()
		if err != nil {
			return "", err
		}

		destPath := filepath.Join(targetDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			src.Close()
			return "", err
		}

		dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			src.Close()
			return "", err
		}

		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		srcCloseErr := src.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if srcCloseErr != nil {
			return "", srcCloseErr
		}
	}

	metas, err := ScanFromDir(targetDir)
	if err != nil {
		return "", err
	}
	if len(metas) == 0 {
		return "", fmt.Errorf("uploaded folder does not contain a valid skill")
	}

	success = true
	return filepath.Base(targetDir), nil
}

func uploadedRootName(files []UploadedFile) (string, error) {
	var rootName string
	for _, file := range files {
		parts, err := splitUploadedPath(file.Path)
		if err != nil {
			return "", err
		}
		if len(parts) < 2 {
			return "", fmt.Errorf("uploaded files must include a top-level folder")
		}
		if rootName == "" {
			rootName = parts[0]
			continue
		}
		if parts[0] != rootName {
			return "", fmt.Errorf("uploaded files must belong to a single folder")
		}
	}
	if rootName == "" {
		return "", fmt.Errorf("uploaded files must include a top-level folder")
	}
	return rootName, nil
}

func normalizeUploadedRelativePath(rawPath, rootName string) (string, error) {
	parts, err := splitUploadedPath(rawPath)
	if err != nil {
		return "", err
	}
	if len(parts) < 2 || parts[0] != rootName {
		return "", fmt.Errorf("uploaded files must belong to folder %s", rootName)
	}
	return path.Join(parts[1:]...), nil
}

func splitUploadedPath(rawPath string) ([]string, error) {
	clean := strings.TrimSpace(strings.ReplaceAll(rawPath, "\\", "/"))
	if clean == "" {
		return nil, fmt.Errorf("uploaded file path is required")
	}
	if strings.HasPrefix(clean, "/") {
		return nil, fmt.Errorf("absolute uploaded paths are not allowed")
	}

	clean = path.Clean(clean)
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || clean == ".." {
		return nil, fmt.Errorf("invalid uploaded file path: %s", rawPath)
	}

	parts := strings.Split(clean, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, fmt.Errorf("invalid uploaded file path: %s", rawPath)
		}
	}
	return parts, nil
}

func uniqueImportDir(root, baseName string) string {
	baseName = strings.TrimSpace(baseName)
	if baseName == "" {
		baseName = "skill"
	}

	candidate := baseName
	for i := 2; ; i++ {
		fullPath := filepath.Join(root, candidate)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return fullPath
		}
		candidate = fmt.Sprintf("%s-%d", baseName, i)
	}
}
