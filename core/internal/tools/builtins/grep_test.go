package builtins

import (
	"strings"
	"testing"
)

func TestGrepToolBasicMatch(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "src/app.go", "package main\n\nfunc Hello() {}\n\nfunc World() {}\n")
	mkfile(t, root, "src/lib.go", "package main\n// no matches here\n")

	tool := NewGrepToolInDir(root)
	out, err := runTool(tool, map[string]any{"pattern": "func [A-Z]"})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	for _, want := range []string{
		"src/app.go:3:1: func Hello() {}",
		"src/app.go:5:1: func World() {}",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "found 2 match") {
		t.Fatalf("expected match count summary, got:\n%s", out)
	}
}

func TestGrepToolCaseInsensitive(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "a.txt", "Hello WORLD\nhello world\n")

	tool := NewGrepToolInDir(root)
	out, err := runTool(tool, map[string]any{
		"pattern":          "world",
		"case_insensitive": true,
	})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, "a.txt:1:") || !strings.Contains(out, "a.txt:2:") {
		t.Fatalf("expected both lines matched case-insensitively, got:\n%s", out)
	}
}

func TestGrepToolGlobFilter(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "src/keep.go", "needle\n")
	mkfile(t, root, "src/skip.ts", "needle\n")

	tool := NewGrepToolInDir(root)
	out, err := runTool(tool, map[string]any{
		"pattern": "needle",
		"glob":    "**/*.go",
	})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, "src/keep.go") {
		t.Fatalf("expected keep.go match, got:\n%s", out)
	}
	if strings.Contains(out, "src/skip.ts") {
		t.Fatalf("ts should be filtered out by glob, got:\n%s", out)
	}
}

func TestGrepToolSkipsBinaryFiles(t *testing.T) {
	root := t.TempDir()
	// Binary: contains a NUL byte in the first sniff window.
	mkfile(t, root, "image.bin", "needle\x00\x01\x02needle")
	mkfile(t, root, "text.txt", "needle\n")

	tool := NewGrepToolInDir(root)
	out, err := runTool(tool, map[string]any{"pattern": "needle"})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, "text.txt") {
		t.Fatalf("expected text.txt match, got:\n%s", out)
	}
	if strings.Contains(out, "image.bin") {
		t.Fatalf("binary file should be skipped, got:\n%s", out)
	}
}

func TestGrepToolSkipsVendoredDirs(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "src/app.go", "needle\n")
	mkfile(t, root, "node_modules/pkg/x.go", "needle\n")
	mkfile(t, root, "dist/bundle.go", "needle\n")

	tool := NewGrepToolInDir(root)
	out, err := runTool(tool, map[string]any{"pattern": "needle"})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, "src/app.go") {
		t.Fatalf("expected src match, got:\n%s", out)
	}
	for _, banned := range []string{"node_modules", "dist/"} {
		if strings.Contains(out, banned) {
			t.Fatalf("%q should be skipped, got:\n%s", banned, out)
		}
	}
}

func TestGrepToolEmptyResultMessage(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "a.txt", "nothing here\n")

	tool := NewGrepToolInDir(root)
	out, err := runTool(tool, map[string]any{"pattern": "needle"})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, "No matches") {
		t.Fatalf("expected no-match hint, got:\n%s", out)
	}
}

func TestGrepToolHonorsMaxResults(t *testing.T) {
	root := t.TempDir()
	var b strings.Builder
	for i := 0; i < 10; i++ {
		b.WriteString("hit\n")
	}
	mkfile(t, root, "many.txt", b.String())

	tool := NewGrepToolInDir(root)
	out, err := runTool(tool, map[string]any{"pattern": "hit", "max_results": 3})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, "stopped at max_results=3") {
		t.Fatalf("expected truncation hint, got:\n%s", out)
	}
	if strings.Count(out, "many.txt:") != 3 {
		t.Fatalf("expected 3 matches before stop, got:\n%s", out)
	}
}

func TestGrepToolSingleFileMode(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "a.txt", "hello\nworld\nhello again\n")

	tool := NewGrepToolInDir(root)
	out, err := runTool(tool, map[string]any{"pattern": "hello", "path": "a.txt"})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, "a.txt:1:1: hello") {
		t.Fatalf("expected single-file match, got:\n%s", out)
	}
	if !strings.Contains(out, "a.txt:3:1: hello again") {
		t.Fatalf("expected second match, got:\n%s", out)
	}
}

func TestGrepToolRejectsInvalidRegex(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "a.txt", "x")
	tool := NewGrepToolInDir(root)
	_, err := runTool(tool, map[string]any{"pattern": "[unclosed"})
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}
