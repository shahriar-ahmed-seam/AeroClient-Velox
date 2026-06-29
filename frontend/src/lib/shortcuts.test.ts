// @vitest-environment jsdom
//
// Unit tests for installGlobalShortcuts (task 17.3).
//
// These tests exercise the window-level keydown listener installed by
// `installGlobalShortcuts`, covering the three application chords and their
// macOS (Meta) equivalents. For each handled chord we assert two things:
//   1. the matching handler runs, and
//   2. the event's default is suppressed (event.defaultPrevented === true),
// which is what Requirements 10.1/10.2/10.5 mean by "suppress the default
// browser handling of the key combination".
//
// We also assert the negative case (an un-modified keypress is ignored and not
// prevented) and that the returned uninstaller actually removes the listener.
//
// This is the first DOM/component test in the suite, so it opts into jsdom via
// the `// @vitest-environment jsdom` header above; the global Vitest
// environment stays 'node' to keep the pure-logic tests fast.
//
// Validates: Requirements 10.1, 10.2, 10.4, 10.5, 10.8, 10.9

import {describe, it, expect, vi, afterEach} from 'vitest'
import {installGlobalShortcuts, type ShortcutHandlers} from './shortcuts'

// Build a set of spy handlers so each test can assert exactly which action ran.
function makeHandlers(): Required<ShortcutHandlers> {
  return {
    onSend: vi.fn(),
    onTogglePalette: vi.fn(),
    onSave: vi.fn(),
  }
}

// Dispatch a keydown on window and return the (already-dispatched) event so the
// caller can read `event.defaultPrevented`. `cancelable: true` is required for
// preventDefault() to actually flip defaultPrevented in jsdom.
function dispatchKey(init: KeyboardEventInit): KeyboardEvent {
  const event = new KeyboardEvent('keydown', {cancelable: true, bubbles: true, ...init})
  window.dispatchEvent(event)
  return event
}

// Track uninstallers so listeners never leak between tests.
let uninstall: (() => void) | null = null
afterEach(() => {
  uninstall?.()
  uninstall = null
  vi.restoreAllMocks()
})

describe('installGlobalShortcuts', () => {
  // Each chord, the modifier that activates it, and the handler it should call.
  // We test both the Ctrl (Windows/Linux) and Meta (macOS) modifier variants.
  const chords: Array<{
    name: string
    key: string
    modifier: 'ctrlKey' | 'metaKey'
    handler: keyof Required<ShortcutHandlers>
  }> = [
    {name: 'Ctrl+Enter', key: 'Enter', modifier: 'ctrlKey', handler: 'onSend'},
    {name: 'Meta+Enter', key: 'Enter', modifier: 'metaKey', handler: 'onSend'},
    {name: 'Ctrl+K', key: 'k', modifier: 'ctrlKey', handler: 'onTogglePalette'},
    {name: 'Cmd+K', key: 'k', modifier: 'metaKey', handler: 'onTogglePalette'},
    {name: 'Ctrl+S', key: 's', modifier: 'ctrlKey', handler: 'onSave'},
    {name: 'Cmd+S', key: 's', modifier: 'metaKey', handler: 'onSave'},
  ]

  for (const chord of chords) {
    it(`${chord.name} calls ${chord.handler} and prevents the default`, () => {
      const handlers = makeHandlers()
      uninstall = installGlobalShortcuts(handlers)

      const event = dispatchKey({key: chord.key, [chord.modifier]: true})

      // The matching handler ran exactly once...
      expect(handlers[chord.handler]).toHaveBeenCalledTimes(1)
      // ...and no other handler fired.
      for (const other of ['onSend', 'onTogglePalette', 'onSave'] as const) {
        if (other !== chord.handler) {
          expect(handlers[other]).not.toHaveBeenCalled()
        }
      }
      // The browser default was suppressed.
      expect(event.defaultPrevented).toBe(true)
    })
  }

  it('treats an uppercase key the same as lowercase (Ctrl+Shift+K)', () => {
    // The listener lower-cases event.key, so a shifted "K" still toggles.
    const handlers = makeHandlers()
    uninstall = installGlobalShortcuts(handlers)

    const event = dispatchKey({key: 'K', ctrlKey: true})

    expect(handlers.onTogglePalette).toHaveBeenCalledTimes(1)
    expect(event.defaultPrevented).toBe(true)
  })

  it('ignores plain Enter (no modifier): no handler, no preventDefault', () => {
    const handlers = makeHandlers()
    uninstall = installGlobalShortcuts(handlers)

    const event = dispatchKey({key: 'Enter'})

    expect(handlers.onSend).not.toHaveBeenCalled()
    expect(handlers.onTogglePalette).not.toHaveBeenCalled()
    expect(handlers.onSave).not.toHaveBeenCalled()
    expect(event.defaultPrevented).toBe(false)
  })

  it('ignores a plain "k" keypress (no modifier)', () => {
    const handlers = makeHandlers()
    uninstall = installGlobalShortcuts(handlers)

    const event = dispatchKey({key: 'k'})

    expect(handlers.onTogglePalette).not.toHaveBeenCalled()
    expect(event.defaultPrevented).toBe(false)
  })

  it('ignores a modified but unbound key (Ctrl+J)', () => {
    const handlers = makeHandlers()
    uninstall = installGlobalShortcuts(handlers)

    const event = dispatchKey({key: 'j', ctrlKey: true})

    expect(handlers.onSend).not.toHaveBeenCalled()
    expect(handlers.onTogglePalette).not.toHaveBeenCalled()
    expect(handlers.onSave).not.toHaveBeenCalled()
    expect(event.defaultPrevented).toBe(false)
  })

  it('the returned uninstaller removes the listener', () => {
    const handlers = makeHandlers()
    const remove = installGlobalShortcuts(handlers)

    // Sanity check: the chord fires while installed.
    dispatchKey({key: 'k', ctrlKey: true})
    expect(handlers.onTogglePalette).toHaveBeenCalledTimes(1)

    remove()

    // After uninstall, the same chord is inert and is not prevented.
    const event = dispatchKey({key: 'k', ctrlKey: true})
    expect(handlers.onTogglePalette).toHaveBeenCalledTimes(1)
    expect(event.defaultPrevented).toBe(false)
  })
})
