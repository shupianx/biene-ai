package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"biene/internal/tools"

	"github.com/bmatcuk/doublestar/v4"
)

const (
	globMaxResults    = 300
	globMaxOutputBytes = 100_000
)

// globSkipDirs are directory names whose entire subtree is skipped during
// the walk. Keeps glob from scanning into vendored / generated trees that
// would dwarf the actual project.
var globSkipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"release":      true,
	"vendor":       true,
	"target":       true,
	"build":        true,
	".cache":       true,
	".next":        true,
	".turbo":       true,
}

// GlobTool finds files by name pattern across the workspace using doublestar
// globbing (`**`, `{a,b}`, etc.).
type GlobTool struct {
	RootDir string
}

func NewGlobTool() *GlobTool                    { return &GlobTool{} }
func NewGlobToolInDir(rootDir string) *GlobTool { return &GlobTool{RootDir: rootDir} }

func (t *GlobTool) Name() string { return "glob" }

func (t *GlobTool) PermissionKey() tools.PermissionKey { return tools.PermissionNone }

func (t *GlobTool) Description() string {
	return `Find files matching a glob pattern (doublestar syntax).
Use this when you know roughly the file's name or extension but not where it lives — much cheaper than list_files when you only want a subset.
Pattern syntax:
  *.go                       — every Go file in the searched directory
  **/*.go                    — every Go file at any depth
  src/**/*.{ts,tsx,vue}      — multi-extension under src
  cmd/*/main.go              — single-segment wildcard
Vendored and generated directories (node_modules, .git, dist, release, vendor, target, build, .cache, .next, .turbo) are skipped automatically.`
}

func (t *GlobTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Glob pattern to match against paths relative to the search root."
			},
			"path": {
				"type": "string",
				"description": "Optional subdirectory to search within (default: workspace root)."
			}
		},
		"required": ["pattern"]
	}`)
}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

func (t *GlobTool) Summary(raw json.RawMessage) string {
	var in globInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "<invalid input>"
	}
	if strings.TrimSpace(in.Path) == "" {
		return in.Pattern
	}
	return fmt.Sprintf("%s in %s", in.Pattern, in.Path)
}

func (t *GlobTool) Execute(_ context.Context, raw json.RawMessage) (string, error) {
	var in globInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid glob input: %w", err)
	}
	pattern := strings.TrimSpace(in.Pattern)
	if pattern == "" {
		return "", fmt.Errorf("glob: pattern is required")
	}
	if !doublestar.ValidatePattern(pattern) {
		return "", fmt.Errorf("glob: invalid pattern %q", pattern)
	}

	searchPath := strings.TrimSpace(in.Path)
	if searchPath == "" {
		searchPath = "."
	}
	rootAbs, _, err := resolvePath(t.RootDir, searchPath)
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}
	info, err := os.Stat(rootAbs)
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("glob: %q is not a directory", searchPath)
	}

	matches, truncated, err := globWalk(rootAbs, pattern)
	if err != nil {
		return "", fmt.Errorf("glob: %w", err)
	}
	if len(matches) == 0 {
		return fmt.Sprintf("No matches for %q.", pattern), nil
	}

	sort.Strings(matches)

	var sb strings.Builder
	for _, rel := range matches {
		sb.WriteString(rel)
		sb.WriteByte('\n')
	}
	if truncated {
		sb.WriteString(fmt.Sprintf("... [truncated at %d matches]\n", globMaxResults))
	}
	out := sb.String()
	if len(out) > globMaxOutputBytes {
		out = out[:globMaxOutputBytes] + fmt.Sprintf("\n... [truncated at %d chars]", globMaxOutputBytes)
	}
	return out, nil
}

// globWalk traverses rootAbs, matching every entry's relative path against
// pattern using doublestar. Skipped directories never enter the walk so
// huge subtrees don't pay any cost.
func globWalk(rootAbs, pattern string) ([]string, bool, error) {
	var (
		matches    []string
		truncated  bool
	)
	err := filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Permission errors on a single dir shouldn't abort the whole search.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path == rootAbs {
			return nil
		}
		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return nil
		}
		slashRel := filepath.ToSlash(rel)

		if d.IsDir() {
			name := d.Name()
			if globSkipDirs[name] {
				return fs.SkipDir
			}
			if tools.IsReservedWorkspacePath(slashRel) {
				return fs.SkipDir
			}
			return nil
		}
		if tools.IsReservedWorkspacePath(slashRel) {
			return nil
		}

		ok, err := doublestar.PathMatch(pattern, slashRel)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		matches = append(matches, slashRel)
		if len(matches) >= globMaxResults {
			truncated = true
			return fs.SkipAll
		}
		return nil
	})
	if err != nil && err != fs.SkipAll {
		return nil, false, err
	}
	return matches, truncated, nil
}
