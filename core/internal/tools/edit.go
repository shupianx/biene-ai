package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// FileEditTool replaces an exact string in a file with a new string.
// The old_string must appear exactly once in the file.
type FileEditTool struct {
	RootDir string
}

func NewFileEditTool() *FileEditTool                    { return &FileEditTool{} }
func NewFileEditToolInDir(rootDir string) *FileEditTool { return &FileEditTool{RootDir: rootDir} }

func (t *FileEditTool) Name() string { return "Edit" }

func (t *FileEditTool) PermissionKey() PermissionKey { return PermissionWrite }

func (t *FileEditTool) Description() string {
	return `Make a precise edit to a file by replacing old_string with new_string.
old_string MUST appear exactly once in the file — include enough surrounding context to make it unique.
The file must have been read with the Read tool before using Edit.
Use Write to create new files or completely rewrite existing ones.`
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

func (t *FileEditTool) IsReadOnly() bool { return false }

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
		return "", fmt.Errorf("invalid Edit input: %w", err)
	}
	if in.FilePath == "" {
		return "", fmt.Errorf("Edit: file_path is required")
	}
	if in.OldString == "" {
		return "", fmt.Errorf("Edit: old_string is required")
	}

	resolvedPath, relPath, err := resolvePath(t.RootDir, in.FilePath)
	if err != nil {
		return "", fmt.Errorf("Edit: %w", err)
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("Edit: reading file: %w", err)
	}
	content := string(data)

	// Verify old_string appears exactly once
	count := strings.Count(content, in.OldString)
	switch count {
	case 0:
		return "", fmt.Errorf("Edit: old_string not found in %s. Make sure it matches the file content exactly (including whitespace and newlines)", relPath)
	default:
		return "", fmt.Errorf("Edit: old_string appears %d times in %s — add more surrounding context to make it unique", count, relPath)
	case 1:
		// exactly one match — proceed
	}

	newContent := strings.Replace(content, in.OldString, in.NewString, 1)
	if err := os.WriteFile(resolvedPath, []byte(newContent), 0o644); err != nil {
		return "", fmt.Errorf("Edit: writing file: %w", err)
	}

	// Report a short diff summary
	oldLines := strings.Count(in.OldString, "\n") + 1
	newLines := strings.Count(in.NewString, "\n") + 1
	return fmt.Sprintf("Successfully edited %s: replaced %d line(s) with %d line(s)", relPath, oldLines, newLines), nil
}
