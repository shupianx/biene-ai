package builtins

import (
	"context"
	"encoding/json"
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
