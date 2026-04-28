package session

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"biene/internal/api"
	"biene/internal/compaction"
	"biene/internal/config"
)

// compactionState holds session-level transient bookkeeping the policy
// reads to avoid double-compacting in race conditions or compacting
// during a manual run that's already underway.
type compactionState struct {
	inFlight bool
}

// resolveCompactionSettings folds the global config defaults and the
// per-session model entry's context window into the local Settings
// shape used by the compaction package.
func (s *Session) resolveCompactionSettings(cfg *config.Config) compaction.Settings {
	base := cfg.CompactionSettings()
	contextWindow := DefaultContextWindow
	if cfg != nil {
		if entry, err := cfg.GetModel(s.modelID); err == nil && entry.ContextWindow > 0 {
			contextWindow = entry.ContextWindow
		}
	}
	return compaction.Settings{
		Enabled:          base.Enabled,
		ReserveTokens:    base.ReserveTokens,
		KeepRecentTokens: base.KeepRecentTokens,
		ContextWindow:    contextWindow,
	}
}

// DefaultContextWindow mirrors config.DefaultContextWindow at the
// session boundary so callers don't have to import the config package.
const DefaultContextWindow = config.DefaultContextWindow

// beforeIteration is the agentloop hook. It runs on the agentloop
// goroutine before each model call. When `lastUsage` indicates the
// input window is approaching the ceiling, it invokes compaction and
// returns the rewritten message list.
//
// On failure the session's history is left untouched; the agent loop
// continues with the original messages. The next iteration may try
// again if usage stays above threshold (subject to the inFlight guard).
func (s *Session) beforeIteration(ctx context.Context, settingsCfg *config.Config) func(ctx context.Context, msgs []api.Message, lastUsage api.Usage) ([]api.Message, error) {
	return func(loopCtx context.Context, msgs []api.Message, lastUsage api.Usage) ([]api.Message, error) {
		settings := s.resolveCompactionSettings(settingsCfg)
		if !compaction.ShouldCompact(lastUsage.InputTokens, settings) {
			return msgs, nil
		}

		s.mu.Lock()
		if s.compactionInFlight {
			s.mu.Unlock()
			return msgs, nil
		}
		s.compactionInFlight = true
		s.mu.Unlock()

		defer func() {
			s.mu.Lock()
			s.compactionInFlight = false
			s.mu.Unlock()
		}()

		s.send(makeFrame("compaction_start", compactionStartPayload{
			TokensBefore: lastUsage.InputTokens,
		}))

		result, err := compaction.Run(loopCtx, s.provider, msgs, settings, "", lastUsage.InputTokens)
		if err != nil {
			if errors.Is(err, compaction.ErrNothingToCompact) {
				slog.Info("auto-compact skipped: no safe cut point",
					"session_id", s.ID, "input_tokens", lastUsage.InputTokens)
				s.send(makeFrame("compaction_failed", compactionFailedPayload{
					Reason: "no safe cut point in current history; will retry next turn",
				}))
				return msgs, nil
			}
			slog.Error("auto-compact failed",
				"session_id", s.ID, "err", err)
			s.send(makeFrame("compaction_failed", compactionFailedPayload{
				Reason: err.Error(),
			}))
			return msgs, nil
		}

		s.applyCompactionResult(result, false)
		return result.Messages, nil
	}
}

// applyCompactionResult appends a CompactionMarker to display_messages,
// persists it, and broadcasts the compaction_done frame. It does NOT
// touch s.apiMessages — that's owned by the agent loop's cfg.Messages
// during a run, and gets snapshotted back into s.apiMessages by
// finishRun. For manual /compact (where no run is active), the caller
// also updates s.apiMessages directly.
func (s *Session) applyCompactionResult(result *compaction.Result, manual bool) DisplayMessage {
	marker := DisplayMessage{
		ID:         newMsgID(),
		Role:       "system",
		AuthorType: "system",
		AuthorName: "Compaction",
		Compaction: &DisplayCompaction{
			Summary:      result.Summary,
			TokensBefore: result.TokensBefore,
			TokensAfter:  result.TokensAfter,
			Replaced:     result.Replaced,
			Manual:       manual,
		},
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.history = append(s.history, marker)
	s.mu.Unlock()

	s.send(makeFrame("message_added", messageAddedPayload{Message: marker}))
	s.send(makeFrame("compaction_done", compactionDonePayload{
		MessageID:    marker.ID,
		TokensBefore: result.TokensBefore,
		TokensAfter:  result.TokensAfter,
		Replaced:     result.Replaced,
	}))

	s.persistDisplayMessage(marker)

	return marker
}

// ErrNoCompactionNeeded is returned by RunManualCompaction when the
// user asks for a checkpoint but the conversation is short enough that
// the keep-recent budget already covers everything. It is a benign
// "no-op" — callers (the REST handler, the UI) should surface it as
// info, not as a failure.
var ErrNoCompactionNeeded = errors.New("session: no compaction needed; conversation is short enough")

// RunManualCompaction is invoked by the /compact REST endpoint. It
// runs synchronously, returns the resulting marker (or an error), and
// commits the new api_messages immediately when the session is idle.
// Refuses to run when an agent loop is mid-flight to keep the policy
// owner-of-state contract simple.
//
// Returns ErrNoCompactionNeeded — sentinel for the "history fits in
// keep-recent already" case — without firing any UI events; the caller
// turns it into a friendly status response. Real failures still send
// compaction_failed and return the underlying error.
func (s *Session) RunManualCompaction(ctx context.Context, cfg *config.Config, instructions string) (*DisplayMessage, error) {
	s.mu.Lock()
	if s.Status == StatusRunning {
		s.mu.Unlock()
		return nil, errors.New("session is busy; cannot run manual compaction during a turn")
	}
	if s.compactionInFlight {
		s.mu.Unlock()
		return nil, errors.New("a compaction is already in progress")
	}
	currentMsgs := append([]api.Message(nil), s.apiMessages...)
	s.mu.Unlock()

	settings := s.resolveCompactionSettings(cfg)
	// Manual still ignores the enable flag — explicit user intent
	// trumps the global toggle.
	settings.Enabled = true

	// Pre-check before broadcasting any UI events. If the keep-recent
	// budget already covers the whole history, there's literally
	// nothing for the summarizer to do; bail with a benign sentinel
	// so the handler can return "no compaction needed" instead of
	// flashing the spinner and then a failure toast.
	headLen := compaction.SyntheticHeadLength(currentMsgs)
	if compaction.FindCutPoint(currentMsgs[headLen:], settings.KeepRecentTokens) == 0 {
		return nil, ErrNoCompactionNeeded
	}

	s.mu.Lock()
	s.compactionInFlight = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.compactionInFlight = false
		s.mu.Unlock()
	}()

	tokensBefore := compaction.EstimateMessagesTokens(currentMsgs)
	s.send(makeFrame("compaction_start", compactionStartPayload{
		TokensBefore: tokensBefore,
	}))

	result, err := compaction.Run(ctx, s.provider, currentMsgs, settings, instructions, tokensBefore)
	if err != nil {
		s.send(makeFrame("compaction_failed", compactionFailedPayload{
			Reason: err.Error(),
		}))
		return nil, err
	}

	marker := s.applyCompactionResult(result, true)

	s.mu.Lock()
	s.apiMessages = result.Messages
	apiSnap := append([]api.Message(nil), s.apiMessages...)
	metaSnap := s.persistentMetaLocked()
	s.mu.Unlock()

	s.persistAfterRun(nil, apiSnap, metaSnap)

	return &marker, nil
}
