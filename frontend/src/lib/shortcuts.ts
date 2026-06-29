// Global keyboard shortcuts (Requirement 10.1, 10.2, 10.5).
//
// `installGlobalShortcuts` registers a single window-level keydown listener that
// implements the three application-wide chords:
//   - Ctrl/Cmd+Enter -> send the current request (requestStore.send)
//   - Ctrl/Cmd+K      -> toggle the command palette (uiStore.toggleCommandPalette)
//   - Ctrl/Cmd+S      -> save the current request (caller-supplied hook)
//
// Every handled chord calls `preventDefault()` so the browser/webview default
// (e.g. the OS save dialog, or a newline) is suppressed. The function returns an
// uninstall callback so the caller (App.svelte) controls the listener lifetime;
// this module never wires itself into App.svelte.

import { requestStore } from './stores/requestStore.svelte'
import { uiStore } from './stores/uiStore.svelte'

/** Optional overrides for the shortcut actions; sensible defaults are used. */
export interface ShortcutHandlers {
  /** Invoked for Ctrl/Cmd+Enter. Defaults to executing the current request. */
  onSend?: () => void
  /** Invoked for Ctrl/Cmd+K. Defaults to toggling the command palette. */
  onTogglePalette?: () => void
  /**
   * Invoked for Ctrl/Cmd+S. Defaults to a no-op hook, since the editor save flow
   * (choosing a target collection) is owned by the sidebar UI; callers can pass a
   * real save action here once one is available.
   */
  onSave?: () => void
}

/**
 * Install the global shortcut listener on `window`. Returns a function that
 * removes the listener. Safe to call in environments without a `window`
 * (e.g. SSR/tests): it becomes a no-op and the returned uninstaller is harmless.
 */
export function installGlobalShortcuts(handlers: ShortcutHandlers = {}): () => void {
  if (typeof window === 'undefined') return () => {}

  const onSend = handlers.onSend ?? (() => void requestStore.send())
  const onTogglePalette = handlers.onTogglePalette ?? (() => uiStore.toggleCommandPalette())
  const onSave = handlers.onSave ?? (() => {})

  const listener = (event: KeyboardEvent): void => {
    // Accept either Control (Windows/Linux) or Command (macOS) as the modifier.
    if (!(event.ctrlKey || event.metaKey)) return

    if (event.key === 'Enter') {
      event.preventDefault()
      onSend()
      return
    }

    const lowerKey = event.key.toLowerCase()
    if (lowerKey === 'k') {
      event.preventDefault()
      onTogglePalette()
      return
    }
    if (lowerKey === 's') {
      event.preventDefault()
      onSave()
    }
  }

  window.addEventListener('keydown', listener)
  return () => window.removeEventListener('keydown', listener)
}
