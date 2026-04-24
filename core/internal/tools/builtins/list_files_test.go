package builtins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListFilesToolListsTopLevelEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}

	tool := NewListFilesToolInDir(root)
	out, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if !strings.Contains(out, "[file] note.txt (5 bytes)") {
		t.Fatalf("expected file entry in output, got:\n%s", out)
	}
	if !strings.Contains(out, "[dir] src/") {
		t.Fatalf("expected dir entry in output, got:\n%s", out)
	}
}

func TestListFilesToolSupportsRecursiveListing(t *testing.T) {
	root := t.TempDir()
	nestedDir := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := NewListFilesToolInDir(root)
	raw, err := json.Marshal(map[string]any{
		"path":      ".",
		"recursive": true,
		"depth":     3,
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out, "[dir] src/") || !strings.Contains(out, "[dir] src/nested/") {
		t.Fatalf("expected recursive directories in output, got:\n%s", out)
	}
	if !strings.Contains(out, "[file] src/nested/main.go (12 bytes)") {
		t.Fatalf("expected recursive file entry in output, got:\n%s", out)
	}
}

func TestFileReadToolSuggestsListForDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	tool := NewFileReadToolInDir(root)
	raw, err := json.Marshal(map[string]any{"file_path": "docs"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = tool.Execute(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error when reading a directory")
	}
	if !strings.Contains(err.Error(), "use list_files to inspect it first") {
		t.Fatalf("expected list_files hint in error, got: %v", err)
	}
}

func TestListFilesToolHidesTinteDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".tinte", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".tinte", "meta.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := NewListFilesToolInDir(root)

	shallow, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if strings.Contains(shallow, ".tinte") {
		t.Fatalf("expected .tinte to be hidden from shallow list, got:\n%s", shallow)
	}
	if !strings.Contains(shallow, "visible.txt") {
		t.Fatalf("expected visible file in output, got:\n%s", shallow)
	}

	raw, err := json.Marshal(map[string]any{
		"path":      ".",
		"recursive": true,
		"depth":     3,
	})
	if err != nil {
		t.Fatal(err)
	}

	recursive, err := tool.Execute(context.Background(), raw)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if strings.Contains(recursive, ".tinte") {
		t.Fatalf("expected .tinte to be hidden from recursive list, got:\n%s", recursive)
	}

	hiddenRaw, err := json.Marshal(map[string]any{"path": ".tinte"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tool.Execute(context.Background(), hiddenRaw)
	if err == nil {
		t.Fatal("expected explicit .tinte listing to be rejected")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved-path error, got: %v", err)
	}
}
