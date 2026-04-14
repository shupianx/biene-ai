package skills

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const skillFileName = "SKILL.md"

// Metadata is the lightweight catalog entry loaded during discovery.
type Metadata struct {
	Name        string
	Description string
	Dir         string
	FilePath    string
}

// Definition is the fully loaded skill, including instructions.
type Definition struct {
	Metadata
	Instructions string
}

// ScanForWorkDir discovers skill metadata under <workDir>/.biene/skills.
func ScanForWorkDir(workDir string) ([]Metadata, error) {
	root := filepath.Join(workDir, ".biene", "skills")
	return ScanFromDir(root)
}

// ScanFromDir discovers valid skill metadata under root.
func ScanFromDir(root string) ([]Metadata, error) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skills root is not a directory: %s", root)
	}

	var metas []Metadata
	seen := make(map[string]struct{})

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || d.Name() != skillFileName {
			return nil
		}

		meta, err := readMetadata(path)
		if err != nil {
			return nil
		}
		key := strings.ToLower(meta.Name)
		if _, ok := seen[key]; ok {
			return nil
		}
		seen[key] = struct{}{}
		metas = append(metas, meta)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Name < metas[j].Name
	})
	return metas, nil
}

// LoadDefinition loads the full skill body for one discovered skill.
func LoadDefinition(meta Metadata) (Definition, error) {
	content, err := os.ReadFile(meta.FilePath)
	if err != nil {
		return Definition{}, err
	}
	return parseFullSkill(meta, string(content))
}

func readMetadata(path string) (Metadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return Metadata{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return Metadata{}, fmt.Errorf("skill %s missing frontmatter", path)
	}

	meta := Metadata{
		Dir:      filepath.Dir(path),
		FilePath: path,
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			break
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.ToLower(key))
		value = trimYAMLScalar(value)
		switch key {
		case "name":
			meta.Name = value
		case "description":
			meta.Description = value
		}
	}

	if err := scanner.Err(); err != nil {
		return Metadata{}, err
	}
	if meta.Name == "" || meta.Description == "" {
		return Metadata{}, fmt.Errorf("skill %s missing required metadata", path)
	}
	return meta, nil
}

func parseFullSkill(meta Metadata, content string) (Definition, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return Definition{}, fmt.Errorf("skill %s missing frontmatter", meta.FilePath)
	}

	rest := strings.TrimPrefix(content, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return Definition{}, fmt.Errorf("skill %s missing frontmatter terminator", meta.FilePath)
	}

	body := strings.TrimSpace(rest[end+5:])
	if body == "" {
		return Definition{}, fmt.Errorf("skill %s missing body", meta.FilePath)
	}

	return Definition{
		Metadata:     meta,
		Instructions: strings.ReplaceAll(body, "{baseDir}", meta.Dir),
	}, nil
}

func trimYAMLScalar(in string) string {
	value := strings.TrimSpace(in)
	value = strings.Trim(value, `"'`)
	return value
}
