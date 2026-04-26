package builtins

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"biene/internal/tools"

	"github.com/bmatcuk/doublestar/v4"
)

const (
	grepDefaultMaxResults = 50
	grepHardMaxResults    = 200
	grepMaxFileSize       = 5 * 1024 * 1024 // 5MB — skip lockfiles, generated bundles
	grepMaxLineLength     = 1024            // truncate displayed line, model doesn't need 100KB minified line
	grepBinarySniff       = 512             // bytes inspected for NUL detection
	grepMaxOutputBytes    = 100_000
)

// GrepTool searches file contents across the workspace using Go regexp.
type GrepTool struct {
	RootDir string
}

func NewGrepTool() *GrepTool                    { return &GrepTool{} }
func NewGrepToolInDir(rootDir string) *GrepTool { return &GrepTool{RootDir: rootDir} }

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) PermissionKey() tools.PermissionKey { return tools.PermissionNone }

func (t *GrepTool) Description() string {
	return `Search for a regular expression across files in the workspace, line by line.
Use this whenever you need to locate code by content (a function name, a string literal, an import path, an error message). Vastly cheaper than read_file when you only need to find where something appears.
Pattern is Go regexp syntax (RE2). Use case_insensitive=true for case folding instead of (?i) prefixes.
Optionally restrict to a subtree (path) or to files matching a glob (e.g., glob="**/*.go"). Vendored / generated directories (node_modules, .git, dist, release, vendor, target, build, .cache, .next, .turbo) are skipped automatically. Binary files and files larger than 5MB are skipped.`
}

func (t *GrepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Regular expression (Go RE2 syntax)."
			},
			"path": {
				"type": "string",
				"description": "Optional subdirectory to search (default: workspace root)."
			},
			"glob": {
				"type": "string",
				"description": "Optional doublestar glob to filter file names before reading (e.g., **/*.go)."
			},
			"case_insensitive": {
				"type": "boolean",
				"description": "Match case-insensitively (default: false)."
			},
			"max_results": {
				"type": "integer",
				"description": "Cap on matches returned (default: 50, hard ceiling: 200)."
			}
		},
		"required": ["pattern"]
	}`)
}

type grepInput struct {
	Pattern         string `json:"pattern"`
	Path            string `json:"path"`
	Glob            string `json:"glob"`
	CaseInsensitive bool   `json:"case_insensitive"`
	MaxResults      int    `json:"max_results"`
}

func (t *GrepTool) Summary(raw json.RawMessage) string {
	var in grepInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "<invalid input>"
	}
	parts := []string{fmt.Sprintf("%q", in.Pattern)}
	if strings.TrimSpace(in.Path) != "" {
		parts = append(parts, "in "+in.Path)
	}
	if strings.TrimSpace(in.Glob) != "" {
		parts = append(parts, "glob="+in.Glob)
	}
	return strings.Join(parts, " ")
}

func (t *GrepTool) Execute(_ context.Context, raw json.RawMessage) (string, error) {
	var in grepInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("invalid grep input: %w", err)
	}
	pattern := in.Pattern
	if pattern == "" {
		return "", fmt.Errorf("grep: pattern is required")
	}
	if in.CaseInsensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("grep: invalid pattern: %w", err)
	}

	maxResults := in.MaxResults
	if maxResults <= 0 {
		maxResults = grepDefaultMaxResults
	}
	if maxResults > grepHardMaxResults {
		maxResults = grepHardMaxResults
	}

	searchPath := strings.TrimSpace(in.Path)
	if searchPath == "" {
		searchPath = "."
	}
	rootAbs, _, err := resolvePath(t.RootDir, searchPath)
	if err != nil {
		return "", fmt.Errorf("grep: %w", err)
	}
	info, err := os.Stat(rootAbs)
	if err != nil {
		return "", fmt.Errorf("grep: %w", err)
	}

	globPat := strings.TrimSpace(in.Glob)
	if globPat != "" && !doublestar.ValidatePattern(globPat) {
		return "", fmt.Errorf("grep: invalid glob %q", globPat)
	}

	if !info.IsDir() {
		// Single-file mode: ignore glob, just scan that one file.
		matches, err := grepFile(rootAbs, filepath.Base(rootAbs), re, maxResults)
		if err != nil {
			return "", fmt.Errorf("grep: %w", err)
		}
		return formatGrepResults(matches, 1, maxResults), nil
	}

	matches, scanned, err := grepWalk(rootAbs, re, globPat, maxResults)
	if err != nil {
		return "", fmt.Errorf("grep: %w", err)
	}
	return formatGrepResults(matches, scanned, maxResults), nil
}

type grepMatch struct {
	Path   string
	Line   int
	Column int
	Text   string
}

func grepWalk(rootAbs string, re *regexp.Regexp, globPat string, maxResults int) ([]grepMatch, int, error) {
	var (
		out     []grepMatch
		scanned int
	)
	err := filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path == rootAbs {
			return nil
		}
		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return nil
		}
		slashRel := filepath.ToSlash(rel)

		if d.IsDir() {
			if globSkipDirs[d.Name()] {
				return fs.SkipDir
			}
			if tools.IsReservedWorkspacePath(slashRel) {
				return fs.SkipDir
			}
			return nil
		}
		if tools.IsReservedWorkspacePath(slashRel) {
			return nil
		}

		if globPat != "" {
			ok, err := doublestar.PathMatch(globPat, slashRel)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
		}

		scanned++
		fileMatches, ferr := grepFile(path, slashRel, re, maxResults-len(out))
		if ferr != nil {
			// Per-file errors (permission, bad UTF-8 boundary) shouldn't abort.
			return nil
		}
		out = append(out, fileMatches...)
		if len(out) >= maxResults {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil && err != fs.SkipAll {
		return nil, scanned, err
	}
	return out, scanned, nil
}

func grepFile(absPath, displayPath string, re *regexp.Regexp, budget int) ([]grepMatch, error) {
	if budget <= 0 {
		return nil, nil
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, nil
	}
	if info.Size() > grepMaxFileSize {
		return nil, nil
	}

	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if isBinary(f) {
		return nil, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var (
		out  []grepMatch
		lineNo int
	)
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		loc := re.FindStringIndex(line)
		if loc == nil {
			continue
		}
		display := line
		if len(display) > grepMaxLineLength {
			display = display[:grepMaxLineLength] + "…"
		}
		out = append(out, grepMatch{
			Path:   displayPath,
			Line:   lineNo,
			Column: loc[0] + 1,
			Text:   display,
		})
		if len(out) >= budget {
			break
		}
	}
	// Scanner errors (huge line over buffer cap) — silently move on.
	return out, nil
}

func isBinary(f *os.File) bool {
	buf := make([]byte, grepBinarySniff)
	n, _ := f.Read(buf)
	return bytes.IndexByte(buf[:n], 0) >= 0
}

func formatGrepResults(matches []grepMatch, scanned, maxResults int) string {
	if len(matches) == 0 {
		return fmt.Sprintf("No matches (scanned %d file(s)).", scanned)
	}
	var sb strings.Builder
	for _, m := range matches {
		fmt.Fprintf(&sb, "%s:%d:%d: %s\n", m.Path, m.Line, m.Column, m.Text)
	}
	if len(matches) >= maxResults {
		fmt.Fprintf(&sb, "... [stopped at max_results=%d]\n", maxResults)
	}
	fmt.Fprintf(&sb, "(found %d match(es) across scanned files)\n", len(matches))
	out := sb.String()
	if len(out) > grepMaxOutputBytes {
		out = out[:grepMaxOutputBytes] + fmt.Sprintf("\n... [truncated at %d chars]", grepMaxOutputBytes)
	}
	return out
}
