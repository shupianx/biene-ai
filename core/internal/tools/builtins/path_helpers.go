package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"biene/internal/tools"
)

// resolvePath confines file access to rootDir when provided.
// It returns both the absolute path and the path relative to rootDir,
// and rejects paths targeting reserved session-state areas.
func resolvePath(rootDir, requestedPath string) (string, string, error) {
	if requestedPath == "" {
		return "", "", fmt.Errorf("path is required")
	}

	if rootDir == "" {
		abs, err := filepath.Abs(requestedPath)
		if err != nil {
			return "", "", err
		}
		return abs, filepath.Base(abs), nil
	}

	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return "", "", fmt.Errorf("resolving workspace root: %w", err)
	}

	var target string
	if filepath.IsAbs(requestedPath) {
		target = filepath.Clean(requestedPath)
	} else {
		target = filepath.Join(rootAbs, requestedPath)
	}

	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", "", fmt.Errorf("resolving path: %w", err)
	}

	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", "", fmt.Errorf("checking path scope: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("path %q escapes workspace root", requestedPath)
	}
	if tools.IsReservedWorkspacePath(rel) {
		return "", "", fmt.Errorf("path %q is reserved for session state", requestedPath)
	}
	if err := tools.ValidateCoworkAccess(rootAbs, rel); err != nil {
		return "", "", err
	}

	return targetAbs, filepath.ToSlash(rel), nil
}
