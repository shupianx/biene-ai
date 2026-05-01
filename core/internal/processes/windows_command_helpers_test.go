package processes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareWindowsCommandLineResolvesBatchFromPath(t *testing.T) {
	cwd := t.TempDir()
	bin := filepath.Join(t.TempDir(), "node bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	npm := filepath.Join(bin, "npm.cmd")
	if err := os.WriteFile(npm, []byte("@echo off\r\n"), 0o644); err != nil {
		t.Fatalf("write npm.cmd: %v", err)
	}

	env := []string{
		"Path=" + bin,
		"PATHEXT=.EXE;.CMD",
	}
	cmdLine, err := prepareWindowsCommandLine("npm", []string{"run", "dev"}, cwd, env)
	if err != nil {
		t.Fatalf("prepareWindowsCommandLine: %v", err)
	}

	if !strings.HasPrefix(cmdLine, `cmd.exe /d /s /c "`) {
		t.Fatalf("expected cmd.exe wrapper, got %q", cmdLine)
	}
	if !strings.Contains(cmdLine, escapeWindowsCmdCommand(npm)) {
		t.Fatalf("expected resolved npm.cmd path in command line, got %q", cmdLine)
	}
	if !strings.Contains(cmdLine, `^"run^" ^"dev^"`) {
		t.Fatalf("expected escaped batch arguments, got %q", cmdLine)
	}
}

func TestPrepareWindowsCommandLineKeepsExecutableDirect(t *testing.T) {
	bin := t.TempDir()
	node := filepath.Join(bin, "node.exe")
	if err := os.WriteFile(node, []byte(""), 0o644); err != nil {
		t.Fatalf("write node.exe: %v", err)
	}

	env := []string{"PATH=" + bin}
	cmdLine, err := prepareWindowsCommandLine("node", []string{"--version"}, "", env)
	if err != nil {
		t.Fatalf("prepareWindowsCommandLine: %v", err)
	}

	if strings.Contains(cmdLine, " /c ") {
		t.Fatalf("did not expect cmd.exe wrapper for .exe, got %q", cmdLine)
	}
	want := buildWindowsCommandLine(node, []string{"--version"})
	if cmdLine != want {
		t.Fatalf("command line = %q, want %q", cmdLine, want)
	}
}

func TestResolveWindowsExecutableUsesCwdBeforePathWhenAllowed(t *testing.T) {
	cwd := t.TempDir()
	pathBin := t.TempDir()
	cwdTool := filepath.Join(cwd, "tool.cmd")
	pathTool := filepath.Join(pathBin, "tool.cmd")
	if err := os.WriteFile(cwdTool, []byte(""), 0o644); err != nil {
		t.Fatalf("write cwd tool: %v", err)
	}
	if err := os.WriteFile(pathTool, []byte(""), 0o644); err != nil {
		t.Fatalf("write path tool: %v", err)
	}

	resolved, err := resolveWindowsExecutable("tool", cwd, []string{"PATH=" + pathBin})
	if err != nil {
		t.Fatalf("resolveWindowsExecutable: %v", err)
	}
	if resolved != cwdTool {
		t.Fatalf("resolved = %q, want cwd tool %q", resolved, cwdTool)
	}
}

func TestResolveWindowsExecutableHonorsNoDefaultCurrentDirectory(t *testing.T) {
	cwd := t.TempDir()
	pathBin := t.TempDir()
	cwdTool := filepath.Join(cwd, "tool.cmd")
	pathTool := filepath.Join(pathBin, "tool.cmd")
	if err := os.WriteFile(cwdTool, []byte(""), 0o644); err != nil {
		t.Fatalf("write cwd tool: %v", err)
	}
	if err := os.WriteFile(pathTool, []byte(""), 0o644); err != nil {
		t.Fatalf("write path tool: %v", err)
	}

	resolved, err := resolveWindowsExecutable("tool", cwd, []string{
		"PATH=" + pathBin,
		"NoDefaultCurrentDirectoryInExePath=1",
	})
	if err != nil {
		t.Fatalf("resolveWindowsExecutable: %v", err)
	}
	if resolved != pathTool {
		t.Fatalf("resolved = %q, want PATH tool %q", resolved, pathTool)
	}
}

// Regression: extensionless `npm` must resolve to `npm.cmd`, not the
// extensionless Unix-shell shim Node.js ships alongside it. CreateProcess
// can't launch the latter and returns "%1 is not a valid Win32 application".
func TestResolveWindowsExecutablePrefersPathextOverBareFile(t *testing.T) {
	bin := t.TempDir()
	shim := filepath.Join(bin, "npm")
	cmd := filepath.Join(bin, "npm.cmd")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	if err := os.WriteFile(cmd, []byte("@echo off\r\n"), 0o644); err != nil {
		t.Fatalf("write cmd: %v", err)
	}

	resolved, err := resolveWindowsExecutable("npm", "", []string{
		"PATH=" + bin,
		"PATHEXT=.EXE;.CMD",
	})
	if err != nil {
		t.Fatalf("resolveWindowsExecutable: %v", err)
	}
	if resolved != cmd {
		t.Fatalf("resolved = %q, want %q (PATHEXT match must win over bare file)", resolved, cmd)
	}
}

func TestBuildWindowsBatchCommandLineEscapesCmdMetaCharacters(t *testing.T) {
	cmdLine := buildWindowsBatchCommandLine(`C:\Program Files\nodejs\npm.cmd`, []string{`a&b`, `100%`, `say "hi"`}, nil)

	for _, want := range []string{
		`C:\Program^ Files\nodejs\npm.cmd`,
		`^"a^&b^"`,
		`^"100^%^"`,
		`^"say^ \^"hi\^"^"`,
	} {
		if !strings.Contains(cmdLine, want) {
			t.Fatalf("expected %q in %q", want, cmdLine)
		}
	}
}

func TestWindowsComSpecFallbacks(t *testing.T) {
	if got := windowsComSpec([]string{`ComSpec=C:\Windows\System32\cmd.exe`}); got != `C:\Windows\System32\cmd.exe` {
		t.Fatalf("ComSpec fallback = %q", got)
	}

	systemRoot := `C:\Windows`
	if got, want := windowsComSpec([]string{`SystemRoot=` + systemRoot}), filepath.Join(systemRoot, "System32", "cmd.exe"); got != want {
		t.Fatalf("SystemRoot fallback = %q, want %q", got, want)
	}
}
