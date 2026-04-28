package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"biene/internal/store"
)

// ErrForkSourceBusy is returned by Fork when the source agent is in
// the middle of a turn. We refuse the operation rather than try to
// snapshot a half-mutated state — the user can wait or interrupt.
var ErrForkSourceBusy = errors.New("session: cannot fork while the source agent is running")

// Fork creates a new session by HEAD-cloning sourceID's workspace.
//
// Flow:
//  1. Refuse if source is running.
//  2. Allocate a new session id + work directory.
//  3. cp -r the source workspace into the new directory, skipping the
//     `cowork/` tree (its symlinks point at OTHER agents' workspaces
//     and shouldn't survive the fork — see CLAUDE.md cowork notes).
//  4. Rewrite the new directory's meta.json: new id, clear pending
//     permission, drop coworks_granted, stamp Extras.forked_from with
//     the parent's id.
//  5. Hydrate the new Session via loadSessionFromDisk and register
//     it on the manager.
//
// The new session shows up in the agents list immediately; the caller
// receives the *Session for any post-fork wiring.
func (m *SessionManager) Fork(sourceID string, newName string) (*Session, error) {
	src := m.Get(sourceID)
	if src == nil {
		return nil, fmt.Errorf("session: source %q not found", sourceID)
	}

	src.mu.Lock()
	srcStatus := src.Status
	srcWorkDir := src.WorkDir
	src.mu.Unlock()
	if srcStatus == StatusRunning {
		return nil, ErrForkSourceBusy
	}

	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, errors.New("session: fork requires a name")
	}
	if m.NameTaken(newName, "") {
		return nil, fmt.Errorf("session: name %q already in use", newName)
	}

	newID := newSessionID()
	newWorkDir := filepath.Join(m.workspaceRoot, newID)

	if err := copyDirSkipping(srcWorkDir, newWorkDir, isCoworkPath); err != nil {
		// Best-effort cleanup — leaving a partially-copied workspace
		// behind would surface as a "ghost" session on next Init.
		_ = os.RemoveAll(newWorkDir)
		return nil, fmt.Errorf("session: copy workspace: %w", err)
	}

	if err := rewriteForkedMeta(newWorkDir, sourceID, newID, newName); err != nil {
		_ = os.RemoveAll(newWorkDir)
		return nil, fmt.Errorf("session: rewrite meta: %w", err)
	}

	sess, err := m.loadSessionFromDisk(newID, newWorkDir)
	if err != nil {
		_ = os.RemoveAll(newWorkDir)
		return nil, fmt.Errorf("session: load fork: %w", err)
	}

	m.mu.Lock()
	m.sessions[newID] = sess
	m.mu.Unlock()

	m.emitSessionCreated(sess.Meta())

	return sess, nil
}

// rewriteForkedMeta loads meta.json from the freshly-copied workspace,
// applies the fork-specific edits, and saves it back. Edits:
//
//   - id           → newID (the meta on disk still points at sourceID)
//   - name         → newName
//   - work_dir     → newWorkDir absolute path
//   - created_at   → time.Now() (this agent came into being just now)
//   - last_active  → time.Now()
//   - pending_permission → nil  (refers to a tool call from the parent's
//     in-flight loop; the new agent's loop hasn't started)
//   - coworks_granted    → nil  (cowork relationships are explicit
//     invitations; not transferable)
//   - active_skills      → nil  (loaded skills were activated within the
//     parent's running conversation; the new agent starts the next turn
//     fresh and can re-load via use_skill if needed)
//   - extras["forked_from"] → "<sourceID>" (renderer-owned key per
//     CLAUDE.md schema rules; backend doesn't read it)
func rewriteForkedMeta(newWorkDir, sourceID, newID, newName string) error {
	storeDir := filepath.Join(newWorkDir, ".biene")
	st, err := store.Open(storeDir)
	if err != nil {
		return err
	}
	defer st.Close()

	var meta SessionMeta
	if err := st.LoadMeta(&meta); err != nil {
		return err
	}

	now := time.Now()
	meta.ID = newID
	meta.Name = newName
	meta.WorkDir = newWorkDir
	meta.Status = StatusIdle
	meta.CreatedAt = now
	meta.LastActive = now
	meta.PendingPermission = nil
	meta.CoworksGranted = nil
	meta.ActiveSkills = nil

	if meta.Extras == nil {
		meta.Extras = make(store.Extras)
	}
	encodedSource, err := json.Marshal(sourceID)
	if err != nil {
		return err
	}
	meta.Extras["forked_from"] = encodedSource

	return st.SaveMeta(meta)
}

// isCoworkPath returns true for paths that should be excluded from the
// fork copy. The argument is the *relative* path inside the source
// workspace — top-level "cowork" or anything under it.
func isCoworkPath(rel string) bool {
	if rel == "cowork" {
		return true
	}
	if strings.HasPrefix(rel, "cowork"+string(os.PathSeparator)) {
		return true
	}
	if strings.HasPrefix(rel, "cowork/") { // defensive across separators
		return true
	}
	return false
}

// copyDirSkipping clones src → dst recursively. `skip(rel)` is queried
// for every entry; returning true prunes that path (and its descendants
// when it's a directory).
//
// Symlinks are intentionally dropped on the floor regardless of skip —
// the fork should never inherit a symlink's target relationship. The
// only known symlinks in agent workspaces today are under cowork/, but
// new symlinks elsewhere would be a defensive failure mode worth
// catching here rather than later.
func copyDirSkipping(src, dst string, skip func(rel string) bool) error {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}

	return filepath.WalkDir(srcAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcAbs, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		if skip != nil && skip(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Drop symlinks defensively. See doc comment.
			return nil
		}

		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyRegularFile(path, target, info.Mode().Perm())
	})
}

func copyRegularFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
