package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileWriteTool creates or overwrites a file with the given content.
type FileWriteTool struct {
	RootDir string
}

func NewFileWriteTool() *FileWriteTool                    { return &FileWriteTool{} }
func NewFileWriteToolInDir(rootDir string) *FileWriteTool { return &FileWriteTool{RootDir: rootDir} }

func (t *FileWriteTool) Name() string { return "Write" }

func (t *FileWriteTool) PermissionKey() PermissionKey { return PermissionWrite }

func (t *FileWriteTool) Description() string {
	return `Write content to a file, creating it (and any missing parent directories) if necessary.
This completely overwrites the existing file content.
Prefer Edit for making targeted changes to existing files.`
}

func (t *FileWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "Absolute or relative path of the file to write"
			},
			"file_text": {
				"type": "string",
				"description": "The complete content to write to the file"
			}
		},
		"required": ["file_path", "file_text"]
	}`)
}

func (t *FileWriteTool) IsReadOnly() bool { return false }

type writeInput struct {
	FilePath string `json:"file_path"`
	FileText string `json:"file_text"`
}

func (t *FileWriteTool) Summary(raw json.RawMessage) string {
	var in writeInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "prepare file write"
	}
	if in.FilePath == "" {
		return "prepare file write"
	}
	return fmt.Sprintf("%s (%d bytes)", in.FilePath, len(in.FileText))
}

func (t *FileWriteTool) Execute(_ context.Context, raw json.RawMessage) (string, error) {
	var in writeInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid Write input: %w", err)
	}
	if in.FilePath == "" {
		return "", fmt.Errorf("Write: file_path is required")
	}

	resolvedPath, relPath, err := resolvePath(t.RootDir, in.FilePath)
	if err != nil {
		return "", fmt.Errorf("Write: %w", err)
	}

	// Create parent directories if needed
	if dir := filepath.Dir(resolvedPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("Write: creating directories: %w", err)
		}
	}

	if err := os.WriteFile(resolvedPath, []byte(in.FileText), 0o644); err != nil {
		return "", fmt.Errorf("Write: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(in.FileText), relPath), nil
}
