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
	return `Make one or more precise edits to a single file by replacing old_string with new_string.
Two input shapes are supported:
  • Single edit:   {file_path, old_string, new_string}
  • Batch edits:   {file_path, edits: [{old_string, new_string}, ...]}
Each old_string MUST appear exactly once in the file at the moment its patch is applied — include enough surrounding context to make it unique. Patches are applied in array order; each one sees the file as left by the previous one.
The file must have been read with read_file before using edit_file.
Use write_file to create new files or completely rewrite existing ones.
All-or-nothing: if any patch fails to match uniquely, no changes are written to disk.
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
				"description": "Single-edit form: the exact string to find and replace (must be unique in the file)"
			},
			"new_string": {
				"type": "string",
				"description": "Single-edit form: the replacement string"
			},
			"edits": {
				"type": "array",
				"description": "Batch-edit form: list of {old_string, new_string} pairs, applied in order",
				"items": {
					"type": "object",
					"properties": {
						"old_string": {"type": "string"},
						"new_string": {"type": "string"}
					},
					"required": ["old_string", "new_string"]
				}
			}
		},
		"required": ["file_path"]
	}`)
}

type editPatch struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

type editInput struct {
	FilePath  string      `json:"file_path"`
	OldString string      `json:"old_string"`
	NewString string      `json:"new_string"`
	Edits     []editPatch `json:"edits"`
}

func (t *FileEditTool) Summary(raw json.RawMessage) string {
	var in editInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "prepare file edit"
	}
	if in.FilePath == "" {
		return "prepare file edit"
	}
	if len(in.Edits) > 0 {
		return fmt.Sprintf("%s: %d edits", in.FilePath, len(in.Edits))
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

	patches, err := normalizeEditPatches(in)
	if err != nil {
		return "", fmt.Errorf("edit_file: %w", err)
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
	totalOldLines, totalNewLines := 0, 0
	for i, p := range patches {
		count := strings.Count(content, p.OldString)
		switch count {
		case 0:
			return "", fmt.Errorf(
				"edit_file: edit #%d old_string not found in %s. Make sure it matches the current file content exactly (later edits see the file as left by earlier ones)",
				i+1, relPath,
			)
		case 1:
			content = strings.Replace(content, p.OldString, p.NewString, 1)
			totalOldLines += strings.Count(p.OldString, "\n") + 1
			totalNewLines += strings.Count(p.NewString, "\n") + 1
		default:
			return "", fmt.Errorf(
				"edit_file: edit #%d old_string appears %d times in %s — add more surrounding context to make it unique",
				i+1, count, relPath,
			)
		}
	}

	if err := os.WriteFile(resolvedPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("edit_file: writing file: %w", err)
	}

	if len(patches) == 1 {
		return fmt.Sprintf(
			"Successfully edited %s: replaced %d line(s) with %d line(s)",
			relPath, totalOldLines, totalNewLines,
		), nil
	}
	return fmt.Sprintf(
		"Successfully applied %d edits to %s: replaced %d line(s) with %d line(s) total",
		len(patches), relPath, totalOldLines, totalNewLines,
	), nil
}

// normalizeEditPatches collapses the two input shapes (legacy single-edit
// vs new edits[]) into one ordered list. If both are provided, edits[]
// wins — the explicit batch form is the more capable shape; the legacy
// fields are tolerated for callers that haven't migrated yet.
func normalizeEditPatches(in editInput) ([]editPatch, error) {
	if len(in.Edits) > 0 {
		for i, p := range in.Edits {
			if p.OldString == "" {
				return nil, fmt.Errorf("edits[%d].old_string is required", i)
			}
		}
		return in.Edits, nil
	}
	if in.OldString == "" {
		return nil, fmt.Errorf("either old_string or non-empty edits[] is required")
	}
	return []editPatch{{OldString: in.OldString, NewString: in.NewString}}, nil
}
