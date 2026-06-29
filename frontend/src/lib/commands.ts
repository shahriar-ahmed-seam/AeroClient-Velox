// Command registry for the Command Palette (Requirement 10).
//
// This module is intentionally PURE: it imports no stores and touches no DOM.
// Commands describe *what* can be run via a `run()` callback supplied by the
// caller, and `filterCommands` is a pure case-insensitive substring filter over
// each command's displayed `title`. Keeping both pure makes the filter directly
// property-testable (task 17.2 / design Property 23) and lets the palette UI own
// all wiring to the Svelte stores.

/** A single executable command shown in the Command Palette. */
export interface Command {
  /** Stable identifier, unique within the registry. */
  id: string
  /** The displayed name; the substring target for palette filtering. */
  title: string
  /** Optional extra search terms (not used by `filterCommands`, reserved for UI). */
  keywords?: string[]
  /** Performs the command's action. */
  run: () => void
}

/**
 * The action callbacks the default command set delegates to. The palette wires
 * these to the real stores so this module stays free of store/DOM dependencies.
 */
export interface CommandHandlers {
  /** Execute the current request (Ctrl/Cmd+Enter equivalent). */
  sendRequest: () => void
  /** Save the current request (Ctrl/Cmd+S equivalent). */
  saveRequest: () => void
  /** Open the Settings view. */
  openSettings: () => void
  /** Open the main workbench view. */
  openWorkbench: () => void
}

/**
 * Build the default command registry from a set of action handlers. Returns a
 * fresh array each call so callers may hold or further transform it freely.
 */
export function createCommands(handlers: CommandHandlers): Command[] {
  return [
    {
      id: 'request.send',
      title: 'Send Request',
      keywords: ['execute', 'run', 'go'],
      run: handlers.sendRequest,
    },
    {
      id: 'request.save',
      title: 'Save Request',
      keywords: ['store', 'persist'],
      run: handlers.saveRequest,
    },
    {
      id: 'view.workbench',
      title: 'Open Workbench',
      keywords: ['editor', 'home', 'request'],
      run: handlers.openWorkbench,
    },
    {
      id: 'view.settings',
      title: 'Open Settings',
      keywords: ['preferences', 'options', 'config'],
      run: handlers.openSettings,
    },
  ]
}

/**
 * Filter `commands` to those whose `title` contains `query` as a
 * case-insensitive substring. An empty (or whitespace-only) query returns all
 * commands. The input order is preserved and the input array is never mutated.
 *
 * Validates: Requirements 10.3, 10.7
 */
export function filterCommands(commands: Command[], query: string): Command[] {
  const needle = query.trim().toLowerCase()
  if (needle === '') return commands.slice()
  return commands.filter((command) => command.title.toLowerCase().includes(needle))
}
