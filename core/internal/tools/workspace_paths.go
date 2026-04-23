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

// SharedRootPrefix is the top-level directory inside a receiver's workspace
// where incoming shares from other agents are exposed as symlinks. Mirrors
// session.SharedRootSubdir; duplicated here so the tools package can check
// paths without importing session.
const SharedRootPrefix = "shared"

// ValidateSharedAccess enforces that any access under shared/ must go
// through a real symlink created by share_to_agent. It accepts:
//   - anything not under shared/
//   - shared/ itself (listing)
//   - shared/<agentID>/ (listing incoming shares from one agent)
//   - shared/<agentID>/<name>[/...] as long as shared/<agentID>/<name>
//     is an existing symlink; this is the share the agent is operating on
//
// It rejects any other form — notably attempts to create a new directory
// or file directly under shared/ that would masquerade as a share.
func ValidateSharedAccess(rootAbs, relPath string) error {
	slash := filepath.ToSlash(relPath)
	if slash == "" || slash == "." {
		return nil
	}
	parts := strings.Split(slash, "/")
	if parts[0] != SharedRootPrefix {
		return nil
	}
	// shared or shared/<agentID>
	if len(parts) <= 2 {
		return nil
	}
	shareRoot := filepath.Join(rootAbs, parts[0], parts[1], parts[2])
	info, err := os.Lstat(shareRoot)
	if err != nil {
		return fmt.Errorf("share %s/%s/%s is not registered", parts[0], parts[1], parts[2])
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s/%s/%s is not a valid share", parts[0], parts[1], parts[2])
	}
	return nil
}
