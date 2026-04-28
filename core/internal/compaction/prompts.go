package compaction

// summarizationSystemPrompt is the standing instruction the summarizer
// model runs under, regardless of whether this is a first-time or update
// pass. The structured schema is the contract — agentloop and the UI
// both assume the section names below.
const summarizationSystemPrompt = `You are a careful summarizer. Your job is to compress a conversation
between a user and an AI assistant so it fits within a smaller context
window without losing decisions, constraints, or open work.

CRITICAL RULES (these are not negotiable):
- Preserve the user's exact words for goals, constraints, and explicit
  preferences. Do not paraphrase phrases like "use X not Y" into
  "the user prefers X". Quote them.
- Be specific: keep file paths, function names, line numbers, dates,
  ticket IDs, and decisions verbatim.
- Do NOT include implementation details that can be re-derived by
  reading the code.
- Do NOT invent information that isn't in the conversation.
- File-tracking sections must list every file path mentioned in tool
  inputs/outputs, deduplicated.

Output the summary in EXACTLY the markdown structure given below.
Do not add or remove top-level sections. Empty sections may use the
literal text "(none)" rather than being omitted.`

// initialSummarizationPrompt instructs the model on first-time
// compaction. The conversation transcript follows this prompt verbatim.
const initialSummarizationPrompt = `Produce a summary of the conversation below using exactly this
markdown structure:

# Conversation Summary

## Goal
<What the user wants to achieve. Quote their exact phrasing if possible.>

## Constraints & Preferences
<Hard constraints, declared preferences, style or tone requirements,
explicit dos and don'ts. Use the user's own words.>

## Progress

### Done
<Concrete completed work. Reference files / functions / commands.>

### In Progress
<Started but not finished.>

### Blocked / Open Questions
<Stuck on, awaiting user input, or known unknowns.>

## Key Decisions
<Format each as: "Decision — Rationale". Include reasoning for choosing X over Y.>

## Next Steps
<What should happen next, in priority order.>

## Critical Context
<Anything else that the next session must know that doesn't fit above:
specific values, IDs, environment quirks, lessons from failed attempts.>

<read-files>
<One file path per line — every file that was read during the
conversation, deduplicated.>
</read-files>

<modified-files>
<One file path per line — every file that was written or edited,
deduplicated.>
</modified-files>

--- BEGIN CONVERSATION TRANSCRIPT ---
%s
--- END CONVERSATION TRANSCRIPT ---`

// updateSummarizationPrompt instructs the model on subsequent
// compactions. The previous summary is included so prior facts persist
// and the file-tracking sections accumulate across rounds.
const updateSummarizationPrompt = `An earlier summary already exists. PRESERVE every fact in the previous
summary unless something below explicitly contradicts or completes it.
ADD any new progress, decisions, constraints, and context discovered in
the new transcript. Output the COMPLETE updated summary using the same
markdown structure — do not shorten existing sections.

Critical: file-tracking sections must accumulate. A file once read
remains in <read-files> forever. A file once modified remains in
<modified-files> forever, even if no further edits occurred.

--- BEGIN PREVIOUS SUMMARY ---
%s
--- END PREVIOUS SUMMARY ---

--- BEGIN NEW CONVERSATION TRANSCRIPT ---
%s
--- END NEW CONVERSATION TRANSCRIPT ---`

// instructionsAddendum appends a free-form user request to the prompt
// when /compact <instructions> was invoked manually.
const instructionsAddendum = `

ADDITIONAL USER INSTRUCTIONS (apply these on top of the rules above):
%s`
