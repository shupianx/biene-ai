package processes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var defaultWindowsPathExts = []string{".com", ".exe", ".bat", ".cmd"}

// prepareWindowsCommandLine resolves PATHEXT-based commands before they reach
// ConPTY's CreateProcess call. Batch files cannot be launched directly by
// CreateProcess, so .bat/.cmd shims are run through cmd.exe while real
// executables continue to run directly under ConPTY.
func prepareWindowsCommandLine(command string, args []string, cwd string, env []string) (string, error) {
	resolved, err := resolveWindowsExecutable(command, cwd, env)
	if err != nil {
		return "", err
	}
	if isWindowsBatchFile(resolved) {
		return buildWindowsBatchCommandLine(resolved, args, env), nil
	}
	return buildWindowsCommandLine(resolved, args), nil
}

func resolveWindowsExecutable(command string, cwd string, env []string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("windows command: command is required")
	}

	exts := windowsPathExts(env)
	if windowsCommandHasPath(command) {
		return resolveWindowsExecutablePath(command, cwd, exts)
	}

	if cwd != "" && !windowsEnvHas(env, "NoDefaultCurrentDirectoryInExePath") {
		if resolved, ok := findWindowsExecutable(filepath.Join(cwd, command), exts); ok {
			return resolved, nil
		}
	}

	for _, dir := range windowsPathDirs(env) {
		if resolved, ok := findWindowsExecutable(filepath.Join(dir, command), exts); ok {
			return resolved, nil
		}
	}

	return "", fmt.Errorf("windows command: executable %q not found in PATH", command)
}

func resolveWindowsExecutablePath(command string, cwd string, exts []string) (string, error) {
	path := command
	if !windowsIsAbsPath(path) && cwd != "" {
		path = filepath.Join(cwd, path)
	}
	if resolved, ok := findWindowsExecutable(path, exts); ok {
		return resolved, nil
	}
	return "", fmt.Errorf("windows command: executable %q not found", command)
}

func findWindowsExecutable(path string, exts []string) (string, bool) {
	if windowsFileExists(path) {
		return path, true
	}
	if windowsPathHasExt(path) {
		for _, ext := range exts {
			candidate := path + ext
			if windowsFileExists(candidate) {
				return candidate, true
			}
		}
		return "", false
	}
	for _, ext := range exts {
		candidate := path + ext
		if windowsFileExists(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func windowsFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func windowsCommandHasPath(command string) bool {
	return strings.ContainsAny(command, `\/`) || windowsIsAbsPath(command)
}

func windowsIsAbsPath(path string) bool {
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`) {
		return true
	}
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		c := path[0]
		return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
	}
	return filepath.IsAbs(path)
}

func windowsPathHasExt(path string) bool {
	ext := filepath.Ext(path)
	return ext != ""
}

func isWindowsBatchFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".bat" || ext == ".cmd"
}

func windowsPathDirs(env []string) []string {
	pathValue := windowsEnvValue(env, "PATH")
	if pathValue == "" {
		return nil
	}
	rawDirs := strings.Split(pathValue, ";")
	dirs := make([]string, 0, len(rawDirs))
	for _, dir := range rawDirs {
		dir = strings.TrimSpace(strings.Trim(dir, `"`))
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func windowsPathExts(env []string) []string {
	pathext := windowsEnvValue(env, "PATHEXT")
	if strings.TrimSpace(pathext) == "" {
		return append([]string(nil), defaultWindowsPathExts...)
	}
	parts := strings.Split(pathext, ";")
	exts := make([]string, 0, len(parts))
	for _, ext := range parts {
		ext = strings.TrimSpace(strings.ToLower(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		exts = append(exts, ext)
	}
	if len(exts) == 0 {
		return append([]string(nil), defaultWindowsPathExts...)
	}
	return exts
}

func windowsEnvValue(env []string, key string) string {
	value := ""
	for _, kv := range env {
		eq := strings.Index(kv, "=")
		if eq <= 0 {
			continue
		}
		if strings.EqualFold(kv[:eq], key) {
			value = kv[eq+1:]
		}
	}
	return value
}

func windowsEnvHas(env []string, key string) bool {
	for _, kv := range env {
		eq := strings.Index(kv, "=")
		if eq <= 0 {
			continue
		}
		if strings.EqualFold(kv[:eq], key) {
			return true
		}
	}
	return false
}

func windowsComSpec(env []string) string {
	if comspec := strings.TrimSpace(windowsEnvValue(env, "ComSpec")); comspec != "" {
		return comspec
	}
	if systemRoot := strings.TrimSpace(windowsEnvValue(env, "SystemRoot")); systemRoot != "" {
		return filepath.Join(systemRoot, "System32", "cmd.exe")
	}
	return "cmd.exe"
}

// buildWindowsCommandLine joins command + args into a single CreateProcess
// command line using the Microsoft argv quoting rules.
func buildWindowsCommandLine(command string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteWindowsArg(command))
	for _, arg := range args {
		parts = append(parts, quoteWindowsArg(arg))
	}
	return strings.Join(parts, " ")
}

func buildWindowsBatchCommandLine(command string, args []string, env []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, escapeWindowsCmdCommand(command))
	doubleEscape := isWindowsNodeModulesCmdShim(command)
	for _, arg := range args {
		parts = append(parts, escapeWindowsCmdArgument(arg, doubleEscape))
	}
	shellCommand := strings.Join(parts, " ")

	return strings.Join([]string{
		quoteWindowsArg(windowsComSpec(env)),
		"/d",
		"/s",
		"/c",
		`"` + shellCommand + `"`,
	}, " ")
}

// quoteWindowsArg mirrors syscall.EscapeArg. It is kept local so the helper
// can be tested on non-Windows hosts.
func quoteWindowsArg(s string) string {
	if len(s) == 0 {
		return `""`
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\\', ' ', '\t':
			var b strings.Builder
			b.Grow(len(s) + 2)
			appendWindowsEscapedArg(&b, s)
			return b.String()
		}
	}
	return s
}

func appendWindowsEscapedArg(b *strings.Builder, s string) {
	if len(s) == 0 {
		b.WriteString(`""`)
		return
	}

	needsBackslash := false
	hasSpace := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\\':
			needsBackslash = true
		case ' ', '\t':
			hasSpace = true
		}
	}

	if !needsBackslash && !hasSpace {
		b.WriteString(s)
		return
	}
	if !needsBackslash {
		b.WriteByte('"')
		b.WriteString(s)
		b.WriteByte('"')
		return
	}

	if hasSpace {
		b.WriteByte('"')
	}
	slashes := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		default:
			slashes = 0
		case '\\':
			slashes++
		case '"':
			for ; slashes > 0; slashes-- {
				b.WriteByte('\\')
			}
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	if hasSpace {
		for ; slashes > 0; slashes-- {
			b.WriteByte('\\')
		}
		b.WriteByte('"')
	}
}

func escapeWindowsCmdCommand(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isWindowsCmdMeta(r) {
			b.WriteByte('^')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func escapeWindowsCmdArgument(s string, doubleEscapeMetaChars bool) string {
	escaped := quoteWindowsArgAlways(s)
	escaped = escapeWindowsCmdCommand(escaped)
	if doubleEscapeMetaChars {
		escaped = escapeWindowsCmdCommand(escaped)
	}
	return escaped
}

func quoteWindowsArgAlways(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')

	slashes := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		default:
			slashes = 0
		case '\\':
			slashes++
		case '"':
			for ; slashes > 0; slashes-- {
				b.WriteByte('\\')
			}
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	for ; slashes > 0; slashes-- {
		b.WriteByte('\\')
	}
	b.WriteByte('"')
	return b.String()
}

func isWindowsCmdMeta(r rune) bool {
	switch r {
	case '(', ')', '[', ']', '%', '!', '^', '"', '`', '<', '>', '&', '|', ';', ',', ' ', '*', '?':
		return true
	default:
		return false
	}
}

func isWindowsNodeModulesCmdShim(path string) bool {
	slash := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	return strings.Contains(slash, "/node_modules/.bin/") && strings.HasSuffix(slash, ".cmd")
}
