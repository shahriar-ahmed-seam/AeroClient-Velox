// @vitest-environment jsdom
//
// E2E-style smoke test for feature reachability (task 19.2).
//
// This test renders the full App shell with a stubbed Backend and asserts that
// every advertised top-level feature is reachable and operational in its
// default state — no placeholder text, and no control that should be usable is
// permanently disabled (Req 11.5, 11.6). It deliberately exercises navigation
// the way a user would: clicking rail buttons, config tabs, and the command
// button, then confirming the corresponding surface renders.
//
// The Backend is replaced with a fake whose read methods resolve to empty state
// (listTree/listEnvironments/listHistory → [], getSettings → defaults) so the
// onMount load() calls settle cleanly and the UI shows its empty-but-functional
// baseline. The store singletons are reset between tests so each case starts
// from a known view state.
//
// Validates: Requirements 11.5, 11.6

import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest'
import {render, cleanup, fireEvent, screen, waitFor} from '@testing-library/svelte'

import {defaultSettings} from '../lib/models'

// A fake Backend whose methods we control. Declared via vi.hoisted so it can be
// referenced inside the hoisted vi.mock factory below. Read methods resolve to
// empty collections/environments/history and default settings; every other
// method is a benign resolving stub so nothing the UI touches rejects.
const mockBackend = vi.hoisted(() => ({
  executeRequest: vi.fn(),
  listTree: vi.fn(),
  saveCollection: vi.fn(),
  renameCollection: vi.fn(),
  deleteCollection: vi.fn(),
  saveFolder: vi.fn(),
  deleteFolder: vi.fn(),
  saveRequest: vi.fn(),
  deleteRequest: vi.fn(),
  moveRequest: vi.fn(),
  listEnvironments: vi.fn(),
  saveEnvironment: vi.fn(),
  deleteEnvironment: vi.fn(),
  setActiveEnvironment: vi.fn(),
  listHistory: vi.fn(),
  clearHistory: vi.fn(),
  migrateLegacyHistory: vi.fn(),
  getSettings: vi.fn(),
  saveSettings: vi.fn(),
  exportCollection: vi.fn(),
  exportEnvironment: vi.fn(),
  importData: vi.fn(),
  appVersion: vi.fn(),
}))

// Replace the platform Backend selector with one that always returns our fake.
// App → stores → getBackend() all resolve to the same module instance, so this
// single mock covers every store the shell wires up on mount.
vi.mock('../lib/backend', () => ({
  getBackend: () => mockBackend,
}))

// Import the stores (which bind the mocked backend) and the App after the mock
// is registered so the wiring picks up the fake.
import {requestStore, uiStore} from '../lib/stores'
import {emptyRawRequest} from '../lib/models'
import App from '../App.svelte'

// Render the wide layout so the sidebar shows inline (≥1024px is "wide").
function setWideViewport(): void {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    writable: true,
    value: 1280,
  })
}

// Bring the shared store singletons back to their default view state so each
// test starts from the same baseline (the workbench, params tab, no response).
function resetStores(): void {
  uiStore.activeView = 'workbench'
  uiStore.activeConfigTab = 'params'
  uiStore.activeResponseTab = 'body'
  uiStore.sidebarCollapsed = false
  uiStore.commandPaletteOpen = false
  uiStore.errorMessage = null
  uiStore.toastMessage = null

  requestStore.current = emptyRawRequest()
  requestStore.response = null
  requestStore.loading = false
}

beforeEach(() => {
  setWideViewport()
  resetStores()

  for (const fn of Object.values(mockBackend)) fn.mockReset()
  // Read methods resolve to an empty-but-valid baseline.
  mockBackend.listTree.mockResolvedValue([])
  mockBackend.listEnvironments.mockResolvedValue([])
  mockBackend.listHistory.mockResolvedValue([])
  mockBackend.getSettings.mockResolvedValue(defaultSettings())
  mockBackend.migrateLegacyHistory.mockResolvedValue(false)
  mockBackend.appVersion.mockResolvedValue('0.0.0-test')
  // Everything else resolves so any incidental call settles cleanly.
  mockBackend.executeRequest.mockResolvedValue({
    status: 200,
    statusText: 'OK',
    headers: [],
    body: '',
    durationMs: 0,
    sizeBytes: 0,
    error: '',
    truncated: false,
  })
  mockBackend.saveCollection.mockResolvedValue(undefined)
  mockBackend.renameCollection.mockResolvedValue(undefined)
  mockBackend.deleteCollection.mockResolvedValue(undefined)
  mockBackend.saveFolder.mockResolvedValue(undefined)
  mockBackend.deleteFolder.mockResolvedValue(undefined)
  mockBackend.saveRequest.mockResolvedValue(undefined)
  mockBackend.deleteRequest.mockResolvedValue(undefined)
  mockBackend.moveRequest.mockResolvedValue(undefined)
  mockBackend.saveEnvironment.mockResolvedValue(undefined)
  mockBackend.deleteEnvironment.mockResolvedValue(undefined)
  mockBackend.setActiveEnvironment.mockResolvedValue(undefined)
  mockBackend.clearHistory.mockResolvedValue(undefined)
  mockBackend.saveSettings.mockResolvedValue(undefined)
})

afterEach(() => {
  cleanup()
  resetStores()
  vi.restoreAllMocks()
})

describe('App smoke test — feature reachability (Req 11.5, 11.6)', () => {
  it('renders the request editor: URL input, method select, and Send button', () => {
    render(App)

    // The core request-building controls are present.
    expect(screen.getByLabelText('Request URL')).toBeTruthy()
    expect(screen.getByLabelText('HTTP method')).toBeTruthy()
    expect(screen.getByRole('button', {name: 'Send'})).toBeTruthy()
  })

  it('shows the response area in its empty state initially', () => {
    render(App)

    // The response pane is mounted and shows the empty-state prompt (Req 4.1).
    expect(screen.getByText('Send a request to see the response.')).toBeTruthy()
  })

  it('makes each request configuration tab reachable and shows its panel', async () => {
    render(App)

    const tablist = screen.getByRole('tablist', {name: 'Request configuration'})
    expect(tablist).toBeTruthy()

    // Parameters (default) — its editable table is present.
    expect(screen.getByLabelText('Enable parameter')).toBeTruthy()

    // Body — clicking the tab swaps in the body editor's default-none panel.
    await fireEvent.click(screen.getByRole('tab', {name: 'Body'}))
    expect(uiStore.activeConfigTab).toBe('body')
    expect(screen.getByText('This request does not send a body.')).toBeTruthy()

    // Headers — its editable header table is present.
    await fireEvent.click(screen.getByRole('tab', {name: 'Headers'}))
    expect(uiStore.activeConfigTab).toBe('headers')
    expect(screen.getByLabelText('Enable header')).toBeTruthy()

    // Authorization — its default-none panel is present.
    await fireEvent.click(screen.getByRole('tab', {name: 'Authorization'}))
    expect(uiStore.activeConfigTab).toBe('auth')
    expect(screen.getByText('This request does not use authorization.')).toBeTruthy()

    // Back to Parameters — the panel returns.
    await fireEvent.click(screen.getByRole('tab', {name: 'Parameters'}))
    expect(uiStore.activeConfigTab).toBe('params')
    expect(screen.getByLabelText('Enable parameter')).toBeTruthy()
  })

  it('makes the sidebar sections reachable from the rail (Collections/History/Environments)', async () => {
    render(App)

    // Collections is the default section.
    expect(screen.getByText('No collections yet.')).toBeTruthy()

    // History rail button → the history section renders.
    await fireEvent.click(screen.getByRole('button', {name: 'History'}))
    expect(screen.getByText('No history yet.')).toBeTruthy()

    // Environments rail button → the environment manager renders.
    await fireEvent.click(screen.getByRole('button', {name: 'Environments'}))
    expect(screen.getByText('No environments yet.')).toBeTruthy()

    // Collections rail button → back to the collections tree.
    await fireEvent.click(screen.getByRole('button', {name: 'Collections'}))
    expect(screen.getByText('No collections yet.')).toBeTruthy()
  })

  it('opens the command palette and shows its search input', async () => {
    render(App)

    // The command-palette overlay is hidden until opened.
    expect(screen.queryByLabelText('Search commands')).toBeNull()

    await fireEvent.click(screen.getByRole('button', {name: 'Command palette'}))

    // The palette opens and its search field is focusable/usable.
    const input = await screen.findByLabelText('Search commands')
    expect(input).toBeTruthy()
  })

  it('reaches the settings view with a theme selector from the rail', async () => {
    render(App)

    await fireEvent.click(screen.getByRole('button', {name: 'Settings'}))

    expect(uiStore.activeView).toBe('settings')
    // The settings surface renders with its heading and theme selector.
    expect(screen.getByRole('heading', {name: 'Settings'})).toBeTruthy()
    expect(screen.getByRole('combobox')).toBeTruthy()
  })

  it('leaves operational controls enabled in the default state', async () => {
    render(App)

    // Send is enabled when not loading (it is only disabled mid-flight).
    const send = screen.getByRole('button', {name: 'Send'}) as HTMLButtonElement
    expect(send.disabled).toBe(false)

    // Every request-configuration tab button is enabled (no dead controls).
    for (const name of ['Parameters', 'Body', 'Headers', 'Authorization']) {
      const tab = screen.getByRole('tab', {name}) as HTMLButtonElement
      expect(tab.disabled).toBe(false)
    }
  })
})
