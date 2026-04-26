package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"biene/internal/tools"
)

// CoworkRootSubdir is the top-level directory inside a receiver's workspace
// where incoming cowork links from other agents land. Each inviting agent
// gets its own subdirectory (cowork/<sourceAgentID>/) to keep namespaces
// isolated.
const CoworkRootSubdir = "cowork"

// windowsDevModeHint is the message returned when CreateCowork fails on
// Windows because Developer Mode is not enabled. Phrased so the LLM can
// relay it to the user verbatim.
const windowsDevModeHint = "cowork requires Windows Developer Mode to create symlinks. " +
	"Enable it at Settings → Privacy & security → For developers → Developer Mode. " +
	"Until then, use send_message_to_agent to copy files instead."

// CreateCowork is the SessionManager side of cowork_with_agent. It validates
// the request, creates the symlink inside the target's workspace, records
// the grant on the source agent, and enqueues a notification message to
// the target. Returns the symlink path relative to the target's workspace.
func (m *SessionManager) CreateCowork(ctx context.Context, fromAgentID, targetAgentID, sourcePath string) (string, error) {
	if fromAgentID == "" || targetAgentID == "" {
		return "", fmt.Errorf("cowork: both agents are required")
	}
	if fromAgentID == targetAgentID {
		return "", fmt.Errorf("cowork: cannot cowork with yourself")
	}
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "", fmt.Errorf("cowork: source_path is required")
	}

	fromSess := m.Get(fromAgentID)
	if fromSess == nil {
		return "", fmt.Errorf("cowork: source agent %q not found", fromAgentID)
	}
	toSess := m.Get(targetAgentID)
	if toSess == nil {
		return "", fmt.Errorf("cowork: target agent %q not found", targetAgentID)
	}

	// Resolve + validate source path stays inside the source workspace
	// and is not under a reserved prefix.
	sourceAbs, _, err := ResolveWorkspacePath(fromSess.WorkDir, sourcePath)
	if err != nil {
		return "", fmt.Errorf("cowork: %w", err)
	}
	if _, err := os.Lstat(sourceAbs); err != nil {
		return "", fmt.Errorf("cowork: source path does not exist: %w", err)
	}

	baseName := filepath.Base(sourceAbs)
	if baseName == "" || baseName == "." || baseName == string(filepath.Separator) {
		return "", fmt.Errorf("cowork: invalid source path")
	}

	destDir := filepath.Join(toSess.WorkDir, CoworkRootSubdir, fromAgentID)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("cowork: preparing receiver directory: %w", err)
	}
	destLink := filepath.Join(destDir, baseName)

	if _, err := os.Lstat(destLink); err == nil {
		return "", fmt.Errorf("cowork: %s is already cowork-linked with %s", baseName, targetAgentID)
	}

	if err := os.Symlink(sourceAbs, destLink); err != nil {
		if isSymlinkPrivilegeError(err) {
			return "", fmt.Errorf("cowork: %s", windowsDevModeHint)
		}
		return "", fmt.Errorf("cowork: creating symlink: %w", err)
	}

	relLink := filepath.ToSlash(filepath.Join(CoworkRootSubdir, fromAgentID, baseName))

	// Record the grant on the source session (persisted via SaveMeta).
	fromSess.mu.Lock()
	fromSess.coworksGranted = append(fromSess.coworksGranted, GrantedCowork{
		TargetAgentID: targetAgentID,
		SourcePath:    sourcePath,
		CreatedAt:     time.Now(),
	})
	persistedMeta := fromSess.persistentMetaLocked()
	meta := fromSess.metaLocked()
	fromSess.mu.Unlock()

	if fromSess.store != nil {
		if err := fromSess.store.SaveMeta(persistedMeta); err != nil {
			// Roll back the symlink so meta and disk stay consistent.
			_ = os.Remove(destLink)
			fromSess.mu.Lock()
			fromSess.coworksGranted = fromSess.coworksGranted[:len(fromSess.coworksGranted)-1]
			fromSess.mu.Unlock()
			return "", fmt.Errorf("cowork: saving grant: %w", err)
		}
	}
	fromSess.notifyMetaChanged(meta)

	// Notify the receiver via its normal agent-to-agent message channel so
	// the conversation reflects the new cowork invitation.
	messageMeta := fromSess.prepareOutboundAgentDelivery(targetAgentID)
	notice := fmt.Sprintf(
		"I invited you to cowork on my %q at %s (read/write). You can read, edit, and create files inside it; changes write back to my workspace.",
		sourcePath, relLink,
	)
	toSess.enqueueAgentInput(fromSess.ID, fromSess.Name, notice, nil, messageMeta)

	return relLink, nil
}

// EndCowork is the SessionManager side of end_cowork_with_agent.
func (m *SessionManager) EndCowork(fromAgentID, targetAgentID, sourcePath string) error {
	if fromAgentID == "" || targetAgentID == "" {
		return fmt.Errorf("end cowork: both agents are required")
	}
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return fmt.Errorf("end cowork: source_path is required")
	}

	fromSess := m.Get(fromAgentID)
	if fromSess == nil {
		return fmt.Errorf("end cowork: source agent %q not found", fromAgentID)
	}

	fromSess.mu.Lock()
	idx := -1
	for i, link := range fromSess.coworksGranted {
		if link.TargetAgentID == targetAgentID && link.SourcePath == sourcePath {
			idx = i
			break
		}
	}
	if idx < 0 {
		fromSess.mu.Unlock()
		return fmt.Errorf("end cowork: no matching cowork found")
	}
	// Snapshot before mutation so we can roll back on failure.
	removed := fromSess.coworksGranted[idx]
	fromSess.coworksGranted = append(fromSess.coworksGranted[:idx], fromSess.coworksGranted[idx+1:]...)
	persistedMeta := fromSess.persistentMetaLocked()
	meta := fromSess.metaLocked()
	fromSess.mu.Unlock()

	if fromSess.store != nil {
		if err := fromSess.store.SaveMeta(persistedMeta); err != nil {
			// Put it back on failure.
			fromSess.mu.Lock()
			fromSess.coworksGranted = append(fromSess.coworksGranted, removed)
			fromSess.mu.Unlock()
			return fmt.Errorf("end cowork: saving meta: %w", err)
		}
	}
	fromSess.notifyMetaChanged(meta)

	// Best-effort removal of the symlink on disk. If the target session is
	// already gone, or the link was removed manually, that's still a valid
	// end state for the sender — the grant is gone, that's what matters.
	toSess := m.Get(targetAgentID)
	if toSess != nil {
		destLink := filepath.Join(toSess.WorkDir, CoworkRootSubdir, fromAgentID, filepath.Base(sourcePath))
		if err := os.Remove(destLink); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("end cowork: removing symlink: %w", err)
		}
	}
	return nil
}

// ListCoworks returns every cowork relationship currently granted by the
// given agent, filtering out entries whose target session no longer exists
// so the caller never sees stale records.
func (m *SessionManager) ListCoworks(fromAgentID string) []tools.CoworkEntry {
	fromSess := m.Get(fromAgentID)
	if fromSess == nil {
		return nil
	}
	fromSess.mu.Lock()
	grants := append([]GrantedCowork(nil), fromSess.coworksGranted...)
	fromSess.mu.Unlock()

	out := make([]tools.CoworkEntry, 0, len(grants))
	for _, g := range grants {
		target := m.Get(g.TargetAgentID)
		if target == nil {
			continue
		}
		out = append(out, tools.CoworkEntry{
			TargetAgentID:   g.TargetAgentID,
			TargetAgentName: target.Name,
			SourcePath:      g.SourcePath,
			CreatedAt:       g.CreatedAt,
		})
	}
	return out
}

// cleanupCoworksForDeletedSession is called when a session is being
// deleted. It removes (a) any symlinks the deleted session created in
// other sessions' workspaces, and (b) any grants in other sessions that
// targeted the deleted session.
func (m *SessionManager) cleanupCoworksForDeletedSession(deletedID string, grantsFromDeleted []GrantedCowork) {
	// (a) Remove symlinks the deleted session placed in peer workspaces.
	for _, g := range grantsFromDeleted {
		toSess := m.Get(g.TargetAgentID)
		if toSess == nil {
			continue
		}
		destLink := filepath.Join(toSess.WorkDir, CoworkRootSubdir, deletedID, filepath.Base(g.SourcePath))
		_ = os.Remove(destLink)
	}

	// (b) Purge grants in other sessions that pointed at the deleted session.
	m.mu.RLock()
	peers := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		if sess.ID == deletedID {
			continue
		}
		peers = append(peers, sess)
	}
	m.mu.RUnlock()

	for _, peer := range peers {
		peer.mu.Lock()
		filtered := peer.coworksGranted[:0]
		changed := false
		for _, g := range peer.coworksGranted {
			if g.TargetAgentID == deletedID {
				changed = true
				continue
			}
			filtered = append(filtered, g)
		}
		if !changed {
			peer.mu.Unlock()
			continue
		}
		peer.coworksGranted = append([]GrantedCowork(nil), filtered...)
		persistedMeta := peer.persistentMetaLocked()
		meta := peer.metaLocked()
		peer.mu.Unlock()

		if peer.store != nil {
			if err := peer.store.SaveMeta(persistedMeta); err != nil {
				// Log-only: cleanup is best-effort, ListCoworks filters
				// stale entries defensively anyway.
				continue
			}
		}
		peer.notifyMetaChanged(meta)
	}
}

// isSymlinkPrivilegeError returns true when err came from a Windows
// CreateSymbolicLink call that failed because the process lacks the
// required privilege. On Unix this always returns false.
func isSymlinkPrivilegeError(err error) bool {
	if runtime.GOOS != "windows" || err == nil {
		return false
	}
	// ERROR_PRIVILEGE_NOT_HELD (1314) is what Windows returns when
	// CreateSymbolicLink is called without admin rights and Developer
	// Mode is off.
	const errorPrivilegeNotHeld = 1314
	var errno syscall.Errno
	if errors.As(err, &errno) && uintptr(errno) == errorPrivilegeNotHeld {
		return true
	}
	// Fallback string match — some wrappers hide the errno. "privilege"
	// only appears in this class of error message.
	return strings.Contains(strings.ToLower(err.Error()), "privilege")
}
