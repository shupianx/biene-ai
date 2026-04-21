package builtins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunCommandToolSummary(t *testing.T) {
	tool := NewRunCommandTool()
	summary := tool.Summary(json.RawMessage(`{"command":"go","args":["test","./..."]}`))
	if summary != "go test ./..." {
		t.Fatalf("unexpected summary: %q", summary)
	}
}

func TestRunCommandToolExecute(t *testing.T) {
	tool := NewRunCommandTool()
	out, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"go","args":["env","GOOS"]}`))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if strings.TrimSpace(out) != runtime.GOOS {
		t.Fatalf("expected GOOS %q, got %q", runtime.GOOS, out)
	}
}

func TestRunCommandToolCwd(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "frontend")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	marker := filepath.Join(subdir, "marker.txt")
	if err := os.WriteFile(marker, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	tool := NewRunCommandToolInDir(root)

	// Without cwd: command runs in root and does not see marker.txt.
	out, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"ls"}`))
	if err != nil {
		t.Fatalf("Execute (root): %v", err)
	}
	if strings.Contains(out, "marker.txt") {
		t.Fatalf("root ls should not list marker.txt, got: %q", out)
	}

	// With cwd: command runs in subdir and sees marker.txt.
	out, err = tool.Execute(context.Background(), json.RawMessage(`{"command":"ls","cwd":"frontend"}`))
	if err != nil {
		t.Fatalf("Execute (cwd): %v", err)
	}
	if !strings.Contains(out, "marker.txt") {
		t.Fatalf("cwd ls should list marker.txt, got: %q", out)
	}
}

func TestRunCommandToolCwdEscapesWorkspace(t *testing.T) {
	root := t.TempDir()
	tool := NewRunCommandToolInDir(root)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"ls","cwd":"../"}`))
	if err == nil {
		t.Fatal("expected error for cwd escaping workspace, got nil")
	}
	if !strings.Contains(err.Error(), "escapes workspace") {
		t.Fatalf("expected workspace-escape error, got: %v", err)
	}
}

func TestRunCommandToolCwdMissing(t *testing.T) {
	root := t.TempDir()
	tool := NewRunCommandToolInDir(root)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"ls","cwd":"does-not-exist"}`))
	if err == nil {
		t.Fatal("expected error for missing cwd, got nil")
	}
}

func TestRunCommandToolMissingExecutable(t *testing.T) {
	tool := NewRunCommandTool()
	out, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"definitely-not-a-real-biene-command"}`))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(out, "definitely-not-a-real-biene-command") {
		t.Fatalf("expected missing executable output, got %q", out)
	}
}
