// Inline tokens the input bar emits on send. Two kinds share the same
// bracket/paren shape, distinguished by their trigger and kind label:
//   @[Name](agent:<ID>)     — agent mention, value is the agent ID
//   /[Name](skill:<Name>)   — skill invocation, value is the skill name
// Both render in the transcript as stylized chips; the inline form is what
// the LLM actually consumes, so send_to_agent / use_skill can extract the
// precise target from the user's message.
export type TokenKind = 'agent' | 'skill'

export const TOKEN_RE = /([@/])\[([^\]]+)\]\((agent|skill):([^)]+)\)/g

// Legacy alias kept for callers that still think in "mentions". Uses the
// same regex — the token kind is distinguished at match time.
export const MENTION_RE = TOKEN_RE

export interface ParsedToken {
  kind: TokenKind
  value: string
  label: string
}

// renderTokenText escapes a plain-text string and swaps inline tokens for
// chip HTML in a single pass. Used for user message bubbles, which don't
// go through markdown.
export function renderTokenText(source: string): string {
  let out = ''
  let last = 0
  for (const match of source.matchAll(TOKEN_RE)) {
    const idx = match.index ?? 0
    if (idx > last) out += escapeHtml(source.slice(last, idx))
    const [, , label, kind, value] = match
    out += chipHtml(kind as TokenKind, value, label)
    last = idx + match[0].length
  }
  if (last < source.length) out += escapeHtml(source.slice(last))
  return out
}

// Back-compat name; still used by MessageItem.
export const renderMentionText = renderTokenText

export function chipHtml(kind: TokenKind, value: string, label: string): string {
  const trigger = kind === 'skill' ? '/' : '@'
  return (
    `<span class="mention-chip kind-${kind}" data-kind="${kind}" ` +
    `data-value="${escapeAttr(value)}" data-label="${escapeAttr(label)}">` +
    `${trigger}${escapeHtml(label)}` +
    `</span>`
  )
}

// Serializes a DOM chip span back to its inline text form. Called by
// InputBar when extracting the editor's content on send.
export function chipToInlineText(el: HTMLElement): string {
  const kind = (el.dataset.kind as TokenKind | undefined) ?? 'agent'
  const value = el.dataset.value ?? ''
  const label = el.dataset.label ?? ''
  const trigger = kind === 'skill' ? '/' : '@'
  return `${trigger}[${label}](${kind}:${value})`
}

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

function escapeAttr(value: string): string {
  return escapeHtml(value)
}
