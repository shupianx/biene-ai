// Slash-command registry.
//
// A command is something the user invokes by typing `/<id>` in the
// input bar. Unlike skills (which become chips embedded in the message
// the model sees), commands are intercepted client-side and execute
// directly — the message is never sent to the model.
//
// Adding a new command:
//   1. Append a `defineCommand({ ... })` entry below.
//   2. Add `command.<id>.name` / `command.<id>.description` keys to
//      every locale under i18n/locales/.
//   3. Implement `execute` against the sessions store.
//
// `hasArgs` controls pick UX:
//   - false: picking from the slash menu executes the command
//     immediately (no extra Enter needed).
//   - true:  picking auto-completes `/id ` into the input and lets the
//     user type arguments before hitting Enter to execute.

import type { useSessionsStore } from '../stores/sessions'

export interface CommandContext {
  sessionId: string
  args: string
  store: ReturnType<typeof useSessionsStore>
}

export interface SlashCommand {
  id: string
  /** i18n key for the display name (resolved at render time so locale
   *  switches re-render the menu). */
  nameKey: string
  /** i18n key for the help text shown in the slash menu's right column. */
  descriptionKey: string
  hasArgs: boolean
  execute: (ctx: CommandContext) => Promise<void> | void
}

function defineCommand(cmd: SlashCommand): SlashCommand {
  return cmd
}

export const builtinCommands: SlashCommand[] = [
  defineCommand({
    id: 'compact',
    nameKey: 'command.compact.name',
    descriptionKey: 'command.compact.description',
    hasArgs: true,
    async execute({ sessionId, args, store }) {
      await store.compact(sessionId, args || undefined)
    },
  }),
  defineCommand({
    id: 'help',
    nameKey: 'command.help.name',
    descriptionKey: 'command.help.description',
    hasArgs: false,
    execute({ sessionId, store }) {
      store.showHelp(sessionId)
    },
  }),
]

/** Lookup helper used by InputBar.onSend to detect "this typed line is
 *  a known command, route it through execute instead of sending as a
 *  regular message". Returns null when the line isn't a command. */
export function parseCommandLine(text: string): { command: SlashCommand; args: string } | null {
  const trimmed = text.trim()
  if (!trimmed.startsWith('/')) return null
  // First whitespace splits id from args. Arg portion may contain
  // newlines and any other characters — pass through verbatim.
  const match = trimmed.match(/^\/([A-Za-z0-9_-]+)(?:\s+([\s\S]*))?$/)
  if (!match) return null
  const [, id, rawArgs] = match
  const command = builtinCommands.find((c) => c.id === id)
  if (!command) return null
  return { command, args: (rawArgs ?? '').trim() }
}
