package compaction

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"biene/internal/api"
)

// ErrNothingToCompact is returned by Run when the keep-recent budget
// already covers the whole history. Callers decide how to surface it:
// the auto path treats it as "skip this round" silently; the manual
// path turns it into a friendly "no compaction needed" status.
var ErrNothingToCompact = errors.New("compaction: nothing to fold above the keep-recent budget")

// Run drives one compaction round. It picks a cut point using the
// keep-recent threshold (FindCutPoint), then folds the discarded prefix
// into a summary via the model.
//
// `tokensBefore` is the API-reported input token count that triggered
// this run; recorded on Result so the UI can show "compressed at N
// tokens".
//
// `instructions` is optional; non-empty values steer the summarizer
// (typically passed only on the manual path, but accepted here for
// symmetry).
func Run(
	ctx context.Context,
	provider api.Provider,
	msgs []api.Message,
	settings Settings,
	instructions string,
	tokensBefore int,
) (*Result, error) {
	headLen := SyntheticHeadLength(msgs)
	tail := msgs[headLen:]

	cutInTail := FindCutPoint(tail, settings.KeepRecentTokens)
	if cutInTail == 0 {
		return nil, ErrNothingToCompact
	}

	discarded := tail[:cutInTail]
	kept := tail[cutInTail:]

	prevSummary := ExtractPreviousSummary(msgs)
	transcript := SerializeTranscript(discarded)
	prompt := buildSummarizationPrompt(prevSummary, transcript, instructions)

	summary, err := callSummarizer(ctx, provider, prompt)
	if err != nil {
		return nil, fmt.Errorf("compaction: summarizer call failed: %w", err)
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return nil, errors.New("compaction: summarizer returned empty response")
	}

	syntheticPair := buildSyntheticPair(summary)
	newMsgs := make([]api.Message, 0, len(syntheticPair)+len(kept))
	newMsgs = append(newMsgs, syntheticPair...)
	newMsgs = append(newMsgs, kept...)

	return &Result{
		Messages:     newMsgs,
		Summary:      summary,
		TokensBefore: tokensBefore,
		TokensAfter:  EstimateMessagesTokens(newMsgs),
		Replaced:     headLen + len(discarded),
	}, nil
}

// buildSummarizationPrompt picks the initial vs update template and
// optionally appends user-supplied instructions.
func buildSummarizationPrompt(prevSummary, transcript, instructions string) string {
	var prompt string
	if prevSummary == "" {
		prompt = fmt.Sprintf(initialSummarizationPrompt, transcript)
	} else {
		prompt = fmt.Sprintf(updateSummarizationPrompt, prevSummary, transcript)
	}
	if strings.TrimSpace(instructions) != "" {
		prompt += fmt.Sprintf(instructionsAddendum, strings.TrimSpace(instructions))
	}
	return prompt
}

// buildSyntheticPair constructs the [user, assistant] head that
// replaces the discarded prefix. The summary is wrapped with sentinel
// tags so the next compaction round can detect and roll it forward.
func buildSyntheticPair(summary string) []api.Message {
	wrapped := SummaryOpenTag + "\n" + summary + "\n" + SummaryCloseTag
	return []api.Message{
		{
			Role: api.RoleUser,
			Content: []api.ContentBlock{
				api.TextBlock{Text: wrapped},
			},
		},
		{
			Role: api.RoleAssistant,
			Content: []api.ContentBlock{
				api.TextBlock{Text: AckText},
			},
		},
	}
}

// callSummarizer streams the summarization request and concatenates
// every text delta. Reasoning, tool_use, and signature deltas are
// dropped — we only want the structured markdown body.
func callSummarizer(ctx context.Context, provider api.Provider, prompt string) (string, error) {
	messages := []api.Message{
		{
			Role:    api.RoleUser,
			Content: []api.ContentBlock{api.TextBlock{Text: prompt}},
		},
	}
	stream, err := provider.Stream(ctx, summarizationSystemPrompt, messages, nil, api.RequestOptions{})
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for ev := range stream {
		switch ev.Type {
		case api.EventTextDelta:
			sb.WriteString(ev.Text)
		case api.EventError:
			if ev.Err != nil {
				return "", ev.Err
			}
		}
	}
	return sb.String(), nil
}
