package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"biene/internal/tools"
)

// FileEditTool replaces an exact string in a file with a new string.
// The old_string must appear exactly once in the file.
type FileEditTool struct {
	RootDir string
}

func NewFileEditTool() *FileEditTool                    { return &FileEditTool{} }
func NewFileEditToolInDir(rootDir string) *FileEditTool { return &FileEditTool{RootDir: rootDir} }

func (t *FileEditTool) Name() string { return "edit_file" }

func (t *FileEditTool) PermissionKey() tools.PermissionKey { return tools.PermissionWrite }

func (t *FileEditTool) Description() string {
	return `Make a precise edit to a file by replacing old_string with new_string.
old_string MUST appear exactly once in the file — include enough surrounding context to make it unique.
The file must have been read with the read_file tool before using edit_file.
Use write_file to create new files or completely rewrite existing ones.
Only use this tool when the user explicitly asks to modify a file. Do not use it to deliver answers or explanations.`
}

func (t *FileEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "Absolute or relative path to the file to edit"
			},
			"old_string": {
				"type": "string",
				"description": "The exact string to find and replace (must be unique in the file)"
			},
			"new_string": {
				"type": "string",
				"description": "The replacement string"
			}
		},
		"required": ["file_path", "old_string", "new_string"]
	}`)
}

type editInput struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (t *FileEditTool) Summary(raw json.RawMessage) string {
	var in editInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "prepare file edit"
	}
	if in.FilePath == "" {
		return "prepare file edit"
	}
	old := in.OldString
	if len(old) > 40 {
		old = old[:37] + "..."
	}
	return fmt.Sprintf("%s: replace %q", in.FilePath, old)
}

func (t *FileEditTool) Execute(_ context.Context, raw json.RawMessage) (string, error) {
	var in editInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid edit_file input: %w", err)
	}
	if in.FilePath == "" {
		return "", fmt.Errorf("edit_file: file_path is required")
	}
	if in.OldString == "" {
		return "", fmt.Errorf("edit_file: old_string is required")
	}

	resolvedPath, relPath, err := resolvePath(t.RootDir, in.FilePath)
	if err != nil {
		return "", fmt.Errorf("edit_file: %w", err)
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("edit_file: reading file: %w", err)
	}
	content := string(data)

	count := strings.Count(content, in.OldString)
	switch count {
	case 0:
		return "", fmt.Errorf("edit_file: old_string not found in %s. Make sure it matches the file content exactly (including whitespace and newlines)", relPath)
	default:
		return "", fmt.Errorf("edit_file: old_string appears %d times in %s — add more surrounding context to make it unique", count, relPath)
	case 1:
	}

	newContent := strings.Replace(content, in.OldString, in.NewString, 1)
	if err := os.WriteFile(resolvedPath, []byte(newContent), 0o644); err != nil {
		return "", fmt.Errorf("edit_file: writing file: %w", err)
	}

	oldLines := strings.Count(in.OldString, "\n") + 1
	newLines := strings.Count(in.NewString, "\n") + 1
	return fmt.Sprintf("Successfully edited %s: replaced %d line(s) with %d line(s)", relPath, oldLines, newLines), nil
}
