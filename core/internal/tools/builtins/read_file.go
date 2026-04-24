package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"tinte/internal/tools"
)

// FileReadTool reads file contents, optionally limited to a line range.
type FileReadTool struct {
	RootDir string
}

func NewFileReadTool() *FileReadTool                    { return &FileReadTool{} }
func NewFileReadToolInDir(rootDir string) *FileReadTool { return &FileReadTool{RootDir: rootDir} }

func (t *FileReadTool) Name() string { return "read_file" }

func (t *FileReadTool) PermissionKey() tools.PermissionKey { return tools.PermissionNone }

func (t *FileReadTool) Description() string {
	return `Read the contents of a file.
Returns the file content with line numbers prefixed (format: "N\tcontent").
Use offset and limit to read a specific range of lines.
Only use this tool when your answer depends on the actual file contents.`
}

func (t *FileReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "Absolute or relative path to the file to read"
			},
			"offset": {
				"type": "integer",
				"description": "Line number to start reading from (1-based, default 1)"
			},
			"limit": {
				"type": "integer",
				"description": "Maximum number of lines to return (default: all)"
			}
		},
		"required": ["file_path"]
	}`)
}

type readInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

func (t *FileReadTool) Summary(raw json.RawMessage) string {
	var in readInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "<invalid input>"
	}
	return in.FilePath
}

func (t *FileReadTool) Execute(_ context.Context, raw json.RawMessage) (string, error) {
	var in readInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid read_file input: %w", err)
	}
	if in.FilePath == "" {
		return "", fmt.Errorf("read_file: file_path is required")
	}

	resolvedPath, _, err := resolvePath(t.RootDir, in.FilePath)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("read_file: %s is a directory; use list_files to inspect it first", in.FilePath)
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	start := 0
	if in.Offset > 1 {
		start = in.Offset - 1
	}
	if start >= len(lines) {
		return "(offset beyond end of file)", nil
	}

	end := len(lines)
	if in.Limit > 0 && start+in.Limit < end {
		end = start + in.Limit
	}

	lines = lines[start:end]

	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, "%d\t%s\n", start+i+1, line)
	}

	const maxOutput = 100_000
	result := sb.String()
	if len(result) > maxOutput {
		result = result[:maxOutput] + fmt.Sprintf("\n... [truncated at %d chars]", maxOutput)
	}
	return result, nil
}
