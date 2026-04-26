package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFileLegacySingleEdit(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "a.txt", "alpha\nbeta\ngamma\n")

	tool := NewFileEditToolInDir(root)
	out, err := runTool(tool, map[string]any{
		"file_path":  "a.txt",
		"old_string": "beta",
		"new_string": "BETA",
	})
	if err != nil {
		t.Fatalf("edit_file: %v", err)
	}
	if !strings.Contains(out, "Successfully edited") {
		t.Fatalf("unexpected output: %s", out)
	}
	got, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(got) != "alpha\nBETA\ngamma\n" {
		t.Fatalf("unexpected file content: %q", string(got))
	}
}

func TestEditFileBatchAppliedInOrder(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "a.txt", "one\ntwo\nthree\n")

	tool := NewFileEditToolInDir(root)
	out, err := runTool(tool, map[string]any{
		"file_path": "a.txt",
		"edits": []map[string]string{
			{"old_string": "one", "new_string": "ONE"},
			{"old_string": "two", "new_string": "TWO"},
			{"old_string": "three", "new_string": "THREE"},
		},
	})
	if err != nil {
		t.Fatalf("edit_file: %v", err)
	}
	if !strings.Contains(out, "applied 3 edits") {
		t.Fatalf("expected batch summary, got: %s", out)
	}
	got, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(got) != "ONE\nTWO\nTHREE\n" {
		t.Fatalf("unexpected content: %q", string(got))
	}
}

func TestEditFileBatchSecondEditSeesFirstResult(t *testing.T) {
	// Edit 1 introduces a string that edit 2 then matches against — proves
	// patches see the file as left by earlier ones, not the original.
	root := t.TempDir()
	mkfile(t, root, "a.txt", "hello world\n")

	tool := NewFileEditToolInDir(root)
	_, err := runTool(tool, map[string]any{
		"file_path": "a.txt",
		"edits": []map[string]string{
			{"old_string": "hello world", "new_string": "MARKER"},
			{"old_string": "MARKER", "new_string": "final"},
		},
	})
	if err != nil {
		t.Fatalf("edit_file: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(got) != "final\n" {
		t.Fatalf("expected sequential application, got: %q", string(got))
	}
}

func TestEditFileBatchAtomicOnFailure(t *testing.T) {
	// Edit 1 valid, edit 2 has a duplicate match → whole call fails, file
	// must remain unchanged on disk.
	root := t.TempDir()
	original := "alpha\ngamma\ngamma\n"
	mkfile(t, root, "a.txt", original)

	tool := NewFileEditToolInDir(root)
	_, err := runTool(tool, map[string]any{
		"file_path": "a.txt",
		"edits": []map[string]string{
			{"old_string": "alpha", "new_string": "ALPHA"},
			{"old_string": "gamma", "new_string": "GAMMA"},
		},
	})
	if err == nil {
		t.Fatal("expected failure on duplicate-match second edit")
	}
	if !strings.Contains(err.Error(), "edit #2") {
		t.Fatalf("expected error to identify failing edit index, got: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(got) != original {
		t.Fatalf("file must not change when any edit fails, got: %q", string(got))
	}
}

func TestEditFileBatchRejectsMissingOldString(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "a.txt", "x\n")

	tool := NewFileEditToolInDir(root)
	_, err := runTool(tool, map[string]any{
		"file_path": "a.txt",
		"edits": []map[string]string{
			{"old_string": "", "new_string": "y"},
		},
	})
	if err == nil {
		t.Fatal("expected error for empty old_string in batch")
	}
}

func TestEditFileEditsArrayWinsOverLegacyFields(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "a.txt", "ROUTE_A\nROUTE_B\n")

	tool := NewFileEditToolInDir(root)
	_, err := runTool(tool, map[string]any{
		"file_path":  "a.txt",
		"old_string": "ROUTE_A", // legacy field — should be ignored
		"new_string": "WRONG",
		"edits": []map[string]string{
			{"old_string": "ROUTE_B", "new_string": "ROUTE_B_FIXED"},
		},
	})
	if err != nil {
		t.Fatalf("edit_file: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(got) != "ROUTE_A\nROUTE_B_FIXED\n" {
		t.Fatalf("legacy fields should have been ignored when edits[] present, got: %q", string(got))
	}
}
