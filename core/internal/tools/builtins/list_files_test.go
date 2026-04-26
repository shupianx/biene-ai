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

func TestCoworkLinkIsTraversableByFileTools(t *testing.T) {
	// Layout:
	//   senderRoot/<shared>/inner.txt   (the actual file)
	//   receiverRoot/cowork/agent-a/<shared> -> senderRoot/<shared>  (symlink)
	// Receiver agent should be able to list, read, and write through the link.

	senderRoot := t.TempDir()
	receiverRoot := t.TempDir()

	sharedDir := filepath.Join(senderRoot, "shared")
	if err := os.Mkdir(sharedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "inner.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkParent := filepath.Join(receiverRoot, "cowork", "agent-a")
	if err := os.MkdirAll(linkParent, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(linkParent, "shared")
	if err := os.Symlink(sharedDir, linkPath); err != nil {
		// Windows without Developer Mode can't create symlinks. The runtime
		// path returns a clear error to the user; nothing for this test to do.
		t.Skipf("skipping: cannot create symlink (likely Windows without Developer Mode): %v", err)
	}

	listTool := NewListFilesToolInDir(receiverRoot)
	listRaw, err := json.Marshal(map[string]any{"path": "cowork/agent-a/shared"})
	if err != nil {
		t.Fatal(err)
	}
	listOut, err := listTool.Execute(context.Background(), listRaw)
	if err != nil {
		t.Fatalf("list_files through cowork link failed: %v", err)
	}
	if !strings.Contains(listOut, "inner.txt") {
		t.Fatalf("expected inner.txt in cowork listing, got:\n%s", listOut)
	}

	readTool := NewFileReadToolInDir(receiverRoot)
	readRaw, err := json.Marshal(map[string]any{"file_path": "cowork/agent-a/shared/inner.txt"})
	if err != nil {
		t.Fatal(err)
	}
	readOut, err := readTool.Execute(context.Background(), readRaw)
	if err != nil {
		t.Fatalf("read_file through cowork link failed: %v", err)
	}
	if !strings.Contains(readOut, "hello") {
		t.Fatalf("expected file contents in read output, got:\n%s", readOut)
	}

	writeTool := NewFileWriteToolInDir(receiverRoot)
	writeRaw, err := json.Marshal(map[string]any{
		"file_path": "cowork/agent-a/shared/new.txt",
		"file_text": "from receiver",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeTool.Execute(context.Background(), writeRaw); err != nil {
		t.Fatalf("write_file through cowork link failed: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(sharedDir, "new.txt"))
	if err != nil {
		t.Fatalf("expected receiver write to land on sender disk: %v", err)
	}
	if string(got) != "from receiver" {
		t.Fatalf("unexpected content on sender disk: %q", string(got))
	}
}

func TestCoworkPathRejectsFakeSymlink(t *testing.T) {
	// A regular directory placed at cowork/<agent>/<name> must NOT be
	// treated as a valid share — only real symlinks created by
	// cowork_with_agent count.
	receiverRoot := t.TempDir()
	fakeShare := filepath.Join(receiverRoot, "cowork", "agent-a", "shared")
	if err := os.MkdirAll(fakeShare, 0o755); err != nil {
		t.Fatal(err)
	}

	listTool := NewListFilesToolInDir(receiverRoot)
	raw, err := json.Marshal(map[string]any{"path": "cowork/agent-a/shared"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := listTool.Execute(context.Background(), raw); err == nil {
		t.Fatal("expected non-symlink cowork path to be rejected")
	}
}

func TestListFilesToolHidesBieneDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".biene", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".biene", "meta.json"), []byte("{}"), 0o644); err != nil {
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
	if strings.Contains(shallow, ".biene") {
		t.Fatalf("expected .biene to be hidden from shallow list, got:\n%s", shallow)
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
	if strings.Contains(recursive, ".biene") {
		t.Fatalf("expected .biene to be hidden from recursive list, got:\n%s", recursive)
	}

	hiddenRaw, err := json.Marshal(map[string]any{"path": ".biene"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tool.Execute(context.Background(), hiddenRaw)
	if err == nil {
		t.Fatal("expected explicit .biene listing to be rejected")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved-path error, got: %v", err)
	}
}
