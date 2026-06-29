// @vitest-environment jsdom
//
// Responsive layout tests for the App shell (task 18.3).
//
// App implements Requirement 12 via a reactive viewport-width tracker:
//   - `<svelte:window bind:innerWidth>` keeps `innerWidth` in sync with the
//     window, and a `$derived` breakpoint maps it to wide (≥1024) / medium
//     (600–1023) / narrow (<600).
//   - The root `.app` element exposes the active breakpoint as
//     `data-breakpoint`, and the sidebar is presented inline on wide, as an
//     overlay drawer on medium, and as a bottom sheet on narrow (the `.overlay`
//     and `.sheet` classes encode this).
//   - A `$effect` collapses the sidebar off wide and expands it on wide, so a
//     resize across a breakpoint reflows the layout.
//
// These tests render App at the representative widths called out by Req 12 and
// assert the breakpoint attribute, the sidebar presentation, reflow on resize,
// and that the always-present controls (rail + top bar) render at 320px.
//
// App's onMount kicks off store loads through the platform Backend and installs
// global shortcuts; in jsdom the real Wails runtime is absent, so the
// `../lib/backend` module is mocked with a fake whose load methods resolve
// cleanly. Full visual-overflow checks aren't possible in jsdom, so the 320px
// case asserts structural presence as a lightweight proxy for "operable at
// 320px".
//
// Validates: Requirements 12.1, 12.2, 12.3, 12.4, 12.5, 12.6

import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest'
import {render, cleanup, waitFor} from '@testing-library/svelte'

// A fake Backend whose load methods resolve so App.onMount() settles cleanly in
// jsdom. Declared via vi.hoisted so it can be referenced inside the hoisted
// vi.mock factory below.
const mockBackend = vi.hoisted(() => {
  const defaultSettings = () => ({
    theme: 'system' as const,
    tlsVerify: true,
    timeoutSeconds: 30,
    proxyUrl: '',
  })
  return {
    executeRequest: vi.fn(),
    listTree: vi.fn(async () => []),
    saveCollection: vi.fn(),
    renameCollection: vi.fn(),
    deleteCollection: vi.fn(),
    saveFolder: vi.fn(),
    deleteFolder: vi.fn(),
    saveRequest: vi.fn(),
    deleteRequest: vi.fn(),
    moveRequest: vi.fn(),
    listEnvironments: vi.fn(async () => []),
    saveEnvironment: vi.fn(),
    deleteEnvironment: vi.fn(),
    setActiveEnvironment: vi.fn(),
    listHistory: vi.fn(async () => []),
    clearHistory: vi.fn(),
    migrateLegacyHistory: vi.fn(),
    getSettings: vi.fn(async () => defaultSettings()),
    saveSettings: vi.fn(),
    exportCollection: vi.fn(),
    exportEnvironment: vi.fn(),
    importData: vi.fn(),
    appVersion: vi.fn(async () => '0.0.0'),
  }
})

// Replace the platform Backend selector with one that always returns our fake,
// so onMount store loads resolve instead of touching the absent Wails runtime.
vi.mock('../lib/backend', () => ({
  getBackend: () => mockBackend,
}))

import App from '../App.svelte'

// -- helpers ----------------------------------------------------------------

/** Force window.innerWidth to a fixed value (jsdom defaults to 1024). */
function setInnerWidth(width: number): void {
  Object.defineProperty(window, 'innerWidth', {
    value: width,
    configurable: true,
    writable: true,
  })
}

/** The root .app element carrying the data-breakpoint attribute. */
function appRoot(container: HTMLElement): HTMLElement {
  const el = container.querySelector<HTMLElement>('.app')
  if (el == null) throw new Error('.app root not rendered')
  return el
}

/** The sidebar <aside>, present in the DOM whenever the workbench is shown. */
function sidebar(container: HTMLElement): HTMLElement {
  const el = container.querySelector<HTMLElement>('.sidebar')
  if (el == null) throw new Error('.sidebar not rendered')
  return el
}

/** Render App at the given viewport width and return the testing-library result. */
function renderAt(width: number) {
  setInnerWidth(width)
  return render(App)
}

// Representative widths from Req 12 paired with their expected breakpoint.
const cases: Array<{width: number; breakpoint: 'narrow' | 'medium' | 'wide'}> = [
  {width: 320, breakpoint: 'narrow'},
  {width: 599, breakpoint: 'narrow'},
  {width: 600, breakpoint: 'medium'},
  {width: 1023, breakpoint: 'medium'},
  {width: 1024, breakpoint: 'wide'},
  {width: 3840, breakpoint: 'wide'},
]

afterEach(() => {
  cleanup()
})

describe('App responsive breakpoint attribute', () => {
  // Req 12.1 / 12.2 / 12.3: the active breakpoint reflects the viewport width.
  for (const {width, breakpoint} of cases) {
    it(`reports data-breakpoint="${breakpoint}" at ${width}px`, () => {
      const {container} = renderAt(width)
      expect(appRoot(container).getAttribute('data-breakpoint')).toBe(breakpoint)
    })
  }
})

describe('App sidebar presentation per breakpoint', () => {
  // Req 12.1: wide presents the sidebar inline (multi-column), not as an overlay.
  for (const width of [1024, 3840]) {
    it(`presents the sidebar inline at ${width}px (wide)`, () => {
      const {container} = renderAt(width)
      const side = sidebar(container)
      expect(side.classList.contains('overlay')).toBe(false)
      expect(side.classList.contains('sheet')).toBe(false)
      // On wide the reflow effect leaves the sidebar expanded/open.
      expect(side.classList.contains('open')).toBe(true)
    })
  }

  // Req 12.2: medium collapses the sidebar into a toggleable overlay drawer.
  for (const width of [600, 1023]) {
    it(`presents the sidebar as an overlay drawer at ${width}px (medium)`, () => {
      const {container} = renderAt(width)
      const side = sidebar(container)
      expect(side.classList.contains('overlay')).toBe(true)
      expect(side.classList.contains('sheet')).toBe(false)
    })
  }

  // Req 12.3: narrow collapses secondary panels into a bottom sheet.
  for (const width of [320, 599]) {
    it(`presents the sidebar as a bottom sheet at ${width}px (narrow)`, () => {
      const {container} = renderAt(width)
      const side = sidebar(container)
      expect(side.classList.contains('sheet')).toBe(true)
      expect(side.classList.contains('overlay')).toBe(true)
    })
  }
})

describe('App reflow on resize', () => {
  // Req 12.4 / 12.5: resizing across a breakpoint reflows the layout, and the
  // bound innerWidth updates the derived breakpoint via the window resize event.
  it('updates data-breakpoint when the viewport is resized across breakpoints', async () => {
    const {container} = renderAt(1280)
    expect(appRoot(container).getAttribute('data-breakpoint')).toBe('wide')

    // Shrink to a narrow viewport and notify listeners, as a real resize would.
    setInnerWidth(320)
    window.dispatchEvent(new Event('resize'))

    await waitFor(() => {
      expect(appRoot(container).getAttribute('data-breakpoint')).toBe('narrow')
    })

    // And reflow back up to wide.
    setInnerWidth(1440)
    window.dispatchEvent(new Event('resize'))

    await waitFor(() => {
      expect(appRoot(container).getAttribute('data-breakpoint')).toBe('wide')
    })
  })
})

describe('App controls remain present at the 320px minimum', () => {
  // Req 12.5 / 12.6: every region stays reachable down to 320px. jsdom can't
  // measure visual overflow, so assert the always-present controls render: the
  // icon rail and the top bar (with its command-palette control).
  it('renders the app root, icon rail, and top bar at 320px', () => {
    const {container} = renderAt(320)

    expect(appRoot(container)).toBeTruthy()

    const rail = container.querySelector('nav.rail[aria-label="Primary"]')
    expect(rail).toBeTruthy()

    const topbar = container.querySelector('header.topbar')
    expect(topbar).toBeTruthy()

    // The command-palette entry point lives in the top bar and must be reachable.
    expect(container.querySelector('.topbar .cmd')).toBeTruthy()
  })
})
