package builtins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobToolMatchesAtAnyDepth(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "main.go", "package main")
	mkfile(t, root, "src/inner/util.go", "package inner")
	mkfile(t, root, "src/inner/util_test.go", "package inner")
	mkfile(t, root, "README.md", "# readme")

	tool := NewGlobToolInDir(root)
	out, err := runTool(tool, map[string]any{"pattern": "**/*.go"})
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, want := range []string{"main.go", "src/inner/util.go", "src/inner/util_test.go"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "README.md") {
		t.Fatalf("README.md should not match **/*.go, got:\n%s", out)
	}
}

func TestGlobToolBraceExpansion(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "a.ts", "")
	mkfile(t, root, "b.tsx", "")
	mkfile(t, root, "c.vue", "")
	mkfile(t, root, "d.go", "")

	tool := NewGlobToolInDir(root)
	out, err := runTool(tool, map[string]any{"pattern": "*.{ts,tsx,vue}"})
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, want := range []string{"a.ts", "b.tsx", "c.vue"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "d.go") {
		t.Fatalf("d.go should not match brace pattern, got:\n%s", out)
	}
}

func TestGlobToolSkipsVendoredDirs(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "src/app.ts", "")
	mkfile(t, root, "node_modules/lib/index.ts", "")
	mkfile(t, root, "dist/bundle.ts", "")
	mkfile(t, root, ".git/HEAD", "")

	tool := NewGlobToolInDir(root)
	out, err := runTool(tool, map[string]any{"pattern": "**/*.ts"})
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if !strings.Contains(out, "src/app.ts") {
		t.Fatalf("expected src/app.ts in output, got:\n%s", out)
	}
	for _, skip := range []string{"node_modules", "dist/bundle.ts"} {
		if strings.Contains(out, skip) {
			t.Fatalf("%q should be skipped, got:\n%s", skip, out)
		}
	}
}

func TestGlobToolHonorsPathParameter(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "src/app.go", "")
	mkfile(t, root, "tests/app.go", "")

	tool := NewGlobToolInDir(root)
	out, err := runTool(tool, map[string]any{"pattern": "*.go", "path": "src"})
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if !strings.Contains(out, "app.go") {
		t.Fatalf("expected match, got:\n%s", out)
	}
	if strings.Contains(out, "tests") {
		t.Fatalf("tests/ should be outside the search root, got:\n%s", out)
	}
}

func TestGlobToolReservedPathsHidden(t *testing.T) {
	// .biene is an opaque session-state namespace from glob/list_files'
	// perspective — agents reach skills via list_skills / use_skill, not
	// by walking the file system. Stay consistent with list_files which
	// hides the whole subtree.
	root := t.TempDir()
	mkfile(t, root, ".biene/meta.json", "{}")
	mkfile(t, root, ".biene/skills/example/SKILL.md", "x")
	mkfile(t, root, "visible.md", "y")

	tool := NewGlobToolInDir(root)
	out, err := runTool(tool, map[string]any{"pattern": "**/*.md"})
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if !strings.Contains(out, "visible.md") {
		t.Fatalf("expected visible.md, got:\n%s", out)
	}
	if strings.Contains(out, ".biene") {
		t.Fatalf(".biene should be hidden, got:\n%s", out)
	}
}

func TestGlobToolEmptyResultMessage(t *testing.T) {
	root := t.TempDir()
	mkfile(t, root, "a.txt", "")

	tool := NewGlobToolInDir(root)
	out, err := runTool(tool, map[string]any{"pattern": "*.go"})
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if !strings.Contains(out, "No matches") {
		t.Fatalf("expected empty-result hint, got:\n%s", out)
	}
}

func TestGlobToolRejectsInvalidPattern(t *testing.T) {
	root := t.TempDir()
	tool := NewGlobToolInDir(root)
	_, err := runTool(tool, map[string]any{"pattern": "[unclosed"})
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────

type executableTool interface {
	Execute(ctx context.Context, raw json.RawMessage) (string, error)
}

func runTool(t executableTool, in map[string]any) (string, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	return t.Execute(context.Background(), raw)
}

func mkfile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
