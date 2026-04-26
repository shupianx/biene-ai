package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"biene/internal/tools"
)

const (
	defaultListDepth = 3
	maxListEntries   = 500
)

// ListFilesTool lists files and directories inside the workspace root.
type ListFilesTool struct {
	RootDir string
}

func NewListFilesTool() *ListFilesTool                    { return &ListFilesTool{} }
func NewListFilesToolInDir(rootDir string) *ListFilesTool { return &ListFilesTool{RootDir: rootDir} }

func (t *ListFilesTool) Name() string { return "list_files" }

func (t *ListFilesTool) PermissionKey() tools.PermissionKey { return tools.PermissionNone }

func (t *ListFilesTool) Description() string {
	return `List files and directories inside the workspace.
Use this before read_file when you do not know the exact file name or location.
Do not use it when you can answer the user directly without inspecting the workspace.`
}

func (t *ListFilesTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Directory or file path to inspect (default: current workspace root)"
			},
			"recursive": {
				"type": "boolean",
				"description": "Whether to walk subdirectories recursively"
			},
			"depth": {
				"type": "integer",
				"description": "Maximum recursion depth when recursive is true (default 3)"
			}
		}
	}`)
}

type listInput struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
	Depth     int    `json:"depth"`
}

func (t *ListFilesTool) Summary(raw json.RawMessage) string {
	var in listInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "."
	}
	path := strings.TrimSpace(in.Path)
	if path == "" {
		path = "."
	}
	if in.Recursive {
		return fmt.Sprintf("%s (recursive)", path)
	}
	return path
}

func (t *ListFilesTool) Execute(_ context.Context, raw json.RawMessage) (string, error) {
	var in listInput
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return "", fmt.Errorf("invalid list_files input: %w", err)
		}
	}

	requestedPath := strings.TrimSpace(in.Path)
	if requestedPath == "" {
		requestedPath = "."
	}

	resolvedPath, relPath, err := resolvePath(t.RootDir, requestedPath)
	if err != nil {
		return "", fmt.Errorf("list_files: %w", err)
	}

	// Stat (not Lstat) so the top-level path follows symlinks. Without
	// this, listing a cowork link (cowork/<agent>/<name>) would short-
	// circuit to a single "[link] ..." entry on platforms where Lstat
	// reports a directory symlink as non-dir (Linux). Children inside
	// the listing still go through DirEntry.Info() (Lstat-equivalent),
	// so nested symlinks keep their "[link]" rendering.
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("list_files: %w", err)
	}

	if !info.IsDir() {
		linkInfo, lerr := os.Lstat(resolvedPath)
		if lerr != nil {
			linkInfo = info
		}
		return formatListEntry(relPath, resolvedPath, linkInfo), nil
	}

	if !in.Recursive {
		return listDirectoryShallow(resolvedPath, relPath)
	}

	maxDepth := in.Depth
	if maxDepth <= 0 {
		maxDepth = defaultListDepth
	}
	return listDirectoryRecursive(resolvedPath, relPath, maxDepth)
}

func listDirectoryShallow(absDir, relDir string) (string, error) {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return "", fmt.Errorf("list_files: reading directory: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Sprintf("(empty directory) %s/", normalizeListRoot(relDir)), nil
	}

	var sb strings.Builder
	count := 0
	for _, entry := range entries {
		childRel := joinListRel(relDir, entry.Name())
		if tools.IsReservedWorkspacePath(childRel) {
			continue
		}
		if count >= maxListEntries {
			sb.WriteString(fmt.Sprintf("... [truncated at %d entries]\n", maxListEntries))
			break
		}

		info, err := entry.Info()
		if err != nil {
			return "", fmt.Errorf("list_files: reading entry info: %w", err)
		}
		sb.WriteString(formatListEntry(childRel, filepath.Join(absDir, entry.Name()), info))
		sb.WriteByte('\n')
		count++
	}
	return truncateListOutput(sb.String()), nil
}

func listDirectoryRecursive(absDir, relDir string, maxDepth int) (string, error) {
	var sb strings.Builder
	count := 0
	baseDepth := strings.Count(filepath.Clean(absDir), string(os.PathSeparator))

	err := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absDir {
			return nil
		}
		childRel := joinListRel(relDir, strings.TrimPrefix(path, absDir))
		if tools.IsReservedWorkspacePath(childRel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		depth := strings.Count(filepath.Clean(path), string(os.PathSeparator)) - baseDepth
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if count >= maxListEntries {
			return fs.SkipAll
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		sb.WriteString(formatListEntry(childRel, path, info))
		sb.WriteByte('\n')
		count++
		return nil
	})
	if err != nil && err != fs.SkipAll {
		return "", fmt.Errorf("list_files: walking directory: %w", err)
	}
	if count == 0 {
		return fmt.Sprintf("(empty directory) %s/", normalizeListRoot(relDir)), nil
	}
	if count >= maxListEntries {
		sb.WriteString(fmt.Sprintf("... [truncated at %d entries]\n", maxListEntries))
	}
	return truncateListOutput(sb.String()), nil
}

func formatListEntry(relPath, absPath string, info os.FileInfo) string {
	relPath = normalizeListRoot(relPath)
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(absPath)
		if err != nil {
			return fmt.Sprintf("[link] %s", relPath)
		}
		return fmt.Sprintf("[link] %s -> %s", relPath, filepath.ToSlash(target))
	case info.IsDir():
		return fmt.Sprintf("[dir] %s/", relPath)
	default:
		return fmt.Sprintf("[file] %s (%d bytes)", relPath, info.Size())
	}
}

func joinListRel(base, suffix string) string {
	suffix = strings.TrimPrefix(suffix, string(os.PathSeparator))
	suffix = filepath.ToSlash(suffix)
	if base == "." || base == "" {
		return suffix
	}
	if suffix == "" {
		return filepath.ToSlash(base)
	}
	return filepath.ToSlash(filepath.Join(base, suffix))
}

func normalizeListRoot(rel string) string {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rel == "" {
		return "."
	}
	return rel
}

func truncateListOutput(result string) string {
	const maxOutput = 100_000
	if len(result) <= maxOutput {
		return result
	}
	return result[:maxOutput] + fmt.Sprintf("\n... [truncated at %d chars]", maxOutput)
}
