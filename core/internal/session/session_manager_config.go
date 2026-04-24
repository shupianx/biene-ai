package session

import (
	"log"
	"strings"

	"tinte/internal/config"
)

func resolveModelEntry(cfg *config.Config, requestedID string) (config.ModelEntry, string, error) {
	requestedID = strings.TrimSpace(requestedID)
	if requestedID != "" {
		if entry, err := cfg.GetModel(requestedID); err == nil {
			return entry, entry.ID, nil
		}
	}

	entry, err := cfg.GetModel("")
	if err != nil {
		return config.ModelEntry{}, "", err
	}
	return entry, entry.ID, nil
}

func (m *SessionManager) ModelUsageCounts() map[string]int {
	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	m.mu.RUnlock()

	counts := make(map[string]int, len(sessions))
	for _, sess := range sessions {
		meta := sess.Meta()
		if meta.ModelID == "" {
			continue
		}
		counts[meta.ModelID]++
	}
	return counts
}

// UpdateConfig replaces the active runtime config and refreshes session providers
// for subsequent turns while preserving each agent's pinned model selection.
func (m *SessionManager) UpdateConfig(cfg *config.Config) error {
	if _, _, err := resolveModelEntry(cfg, ""); err != nil {
		return err
	}

	m.mu.Lock()
	m.cfg = cfg
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	m.mu.Unlock()

	for _, sess := range sessions {
		sess.mu.Lock()
		currentModelID := sess.modelID
		sess.mu.Unlock()

		modelEntry, resolvedID, err := resolveModelEntry(cfg, currentModelID)
		if err != nil {
			return err
		}

		sess.mu.Lock()
		thinkingAvailable := modelEntry.ThinkingAvailable
		thinkingEnabled := thinkingAvailable
		if sess.thinkingAvailable {
			thinkingEnabled = thinkingAvailable && sess.thinkingEnabled
		}
		sess.provider = newProvider(modelEntry)
		sess.modelID = resolvedID
		sess.modelName = modelEntry.Name
		sess.thinkingAvailable = thinkingAvailable
		sess.thinkingEnabled = thinkingEnabled
		sess.thinkingOn = modelEntry.ThinkingOn
		sess.thinkingOff = modelEntry.ThinkingOff
		meta := sess.metaLocked()
		persistedMeta := sess.persistentMetaLocked()
		sess.mu.Unlock()

		if sess.store != nil {
			if err := sess.store.SaveMeta(persistedMeta); err != nil {
				log.Printf("persist session meta after config update for %s: %v", sess.ID, err)
			}
		}
		sess.notifyMetaChanged(meta)
	}

	return nil
}
