package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// reservedWorkspacePrefixes are forward-slash path prefixes inside a
// session workspace that agent file tools may not read or write.
//
// The whole .biene/ namespace is session/product-internal state by
// default (meta.json, history.db[-wal|-shm], assets/, and whatever we
// add later). Subpaths the agent IS allowed to touch are listed in
// allowedReservedPrefixes — today that's only skills/, where agents
// author and edit SKILL.md files.
var reservedWorkspacePrefixes = []string{
	".biene",
}

var allowedReservedPrefixes = []string{
	".biene/skills",
}

// IsReservedWorkspacePath reports whether relPath (a path relative to
// a session workspace root, in either OS or forward-slash form) targets
// a reserved area that agent file tools must not access.
func IsReservedWorkspacePath(relPath string) bool {
	slash := filepath.ToSlash(relPath)
	reserved := false
	for _, prefix := range reservedWorkspacePrefixes {
		if slash == prefix || strings.HasPrefix(slash, prefix+"/") {
			reserved = true
			break
		}
	}
	if !reserved {
		return false
	}
	for _, allow := range allowedReservedPrefixes {
		if slash == allow || strings.HasPrefix(slash, allow+"/") {
			return false
		}
	}
	return true
}

// CoworkRootPrefix is the top-level directory inside a receiver's workspace
// where incoming cowork relationships from other agents are exposed as
// symlinks. Mirrors session.CoworkRootSubdir; duplicated here so the tools
// package can check paths without importing session.
const CoworkRootPrefix = "cowork"

// ValidateCoworkAccess enforces that any access under cowork/ must go
// through a real symlink created by cowork_with_agent. It accepts:
//   - anything not under cowork/
//   - cowork/ itself (listing)
//   - cowork/<agentID>/ (listing incoming coworks from one agent)
//   - cowork/<agentID>/<name>[/...] as long as cowork/<agentID>/<name>
//     is an existing symlink; this is the cowork the agent is operating on
//
// It rejects any other form — notably attempts to create a new directory
// or file directly under cowork/ that would masquerade as a cowork link.
func ValidateCoworkAccess(rootAbs, relPath string) error {
	slash := filepath.ToSlash(relPath)
	if slash == "" || slash == "." {
		return nil
	}
	parts := strings.Split(slash, "/")
	if parts[0] != CoworkRootPrefix {
		return nil
	}
	// cowork or cowork/<agentID>
	if len(parts) <= 2 {
		return nil
	}
	shareRoot := filepath.Join(rootAbs, parts[0], parts[1], parts[2])
	// Use Readlink rather than the ModeSymlink bit. On Windows, os.Lstat
	// only sets ModeSymlink for IO_REPARSE_TAG_SYMLINK; junctions and other
	// reparse tags are reported as ModeIrregular even though Readlink can
	// resolve them. Readlink succeeds on any reparse point with a target,
	// which is exactly the gate we want here.
	if _, err := os.Readlink(shareRoot); err != nil {
		return fmt.Errorf("share %s/%s/%s is not registered", parts[0], parts[1], parts[2])
	}
	return nil
}
