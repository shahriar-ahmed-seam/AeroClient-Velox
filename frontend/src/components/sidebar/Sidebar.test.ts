// @vitest-environment jsdom
//
// Unit tests for sidebar confirmation and error branches (task 16.2).
//
// The three sidebar surfaces — CollectionsTree, HistoryList, and
// EnvironmentManager — route every mutation through the runes stores
// (collectionsStore / historyStore / environmentsStore), which in turn forward
// to the platform Backend via getBackend(). These tests isolate that Backend by
// mocking the `../../lib/backend` module with a fake whose methods are vi.fn()s
// we control, so store behavior is deterministic in jsdom (where the real Wails
// bindings are absent).
//
// Covered branches:
//  - Deleting a non-empty collection prompts for confirmation; the Backend
//    delete runs only when the user confirms, and not when they decline
//    (Req 5.7).
//  - Clearing history prompts for confirmation; the Backend clear runs only on
//    confirm (Req 7.7).
//  - A failing store mutation surfaces through uiStore.errorMessage while the
//    user's in-progress draft input is preserved, never cleared
//    (Req 11.7, 5.9, 7.9).

import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest'
import {render, cleanup, fireEvent, waitFor} from '@testing-library/svelte'

import type {Collection, Environment, HistoryEntry, RawRequest} from '../../lib/models'

// A fake Backend whose methods we control per test. Declared via vi.hoisted so
// it can be referenced inside the hoisted vi.mock factory below.
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
vi.mock('../../lib/backend', () => ({
  getBackend: () => mockBackend,
}))

// Stores import the mocked backend, so import them after vi.mock is registered.
import {collectionsStore} from '../../lib/stores/collectionsStore.svelte'
import {historyStore} from '../../lib/stores/historyStore.svelte'
import {environmentsStore} from '../../lib/stores/environmentsStore.svelte'
import {uiStore} from '../../lib/stores/uiStore.svelte'

import CollectionsTree from './CollectionsTree.svelte'
import HistoryList from './HistoryList.svelte'
import EnvironmentManager from './EnvironmentManager.svelte'

// -- model builders ---------------------------------------------------------

function rawRequest(): RawRequest {
  return {
    method: 'GET',
    url: 'https://example.com',
    params: [],
    headers: [],
    body: {type: 'none', raw: '', formFields: []},
    auth: {
      type: 'none',
      bearerToken: '',
      basicUser: '',
      basicPass: '',
      apiKeyName: '',
      apiKeyValue: '',
      apiKeyLocation: 'header',
    },
  }
}

/** A collection that holds one (empty) folder, so it counts as non-empty. */
function nonEmptyCollection(): Collection {
  return {
    id: 'col-1',
    name: 'My Collection',
    folders: [{id: 'fold-1', name: 'Sub', folders: [], requests: []}],
    requests: [],
    order: 0,
  }
}

function historyEntry(): HistoryEntry {
  return {
    id: 'hist-1',
    method: 'GET',
    url: 'https://example.com',
    status: 200,
    durationMs: 12,
    at: Date.now(),
    error: '',
    request: rawRequest(),
  }
}

// Find a button by its trimmed visible text.
function buttonByText(container: HTMLElement, text: string): HTMLButtonElement {
  const btn = Array.from(container.querySelectorAll('button')).find(
    (b) => b.textContent?.trim() === text,
  )
  if (!btn) throw new Error(`button with text "${text}" not found`)
  return btn as HTMLButtonElement
}

// -- shared reset -----------------------------------------------------------

beforeEach(() => {
  // Reset every fake Backend method and provide benign defaults so reload
  // calls after a successful mutation resolve cleanly.
  for (const fn of Object.values(mockBackend)) fn.mockReset()
  mockBackend.listTree.mockResolvedValue([])
  mockBackend.listHistory.mockResolvedValue([])
  mockBackend.listEnvironments.mockResolvedValue([])
  mockBackend.deleteCollection.mockResolvedValue(undefined)
  mockBackend.deleteFolder.mockResolvedValue(undefined)
  mockBackend.clearHistory.mockResolvedValue(undefined)
  mockBackend.saveEnvironment.mockResolvedValue(undefined)

  // Reset shared store state and any lingering error between tests.
  collectionsStore.tree = []
  historyStore.entries = []
  environmentsStore.environments = []
  uiStore.errorMessage = null
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

// ---------------------------------------------------------------------------

describe('CollectionsTree delete confirmation (Req 5.7)', () => {
  it('deletes a non-empty collection only after the user confirms', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    collectionsStore.tree = [nonEmptyCollection()]

    const {container} = render(CollectionsTree)

    // The collection-level Delete button is the first one in document order.
    const deleteBtn = container.querySelectorAll<HTMLButtonElement>('[title="Delete"]')[0]
    await fireEvent.click(deleteBtn)

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    await waitFor(() => {
      expect(mockBackend.deleteCollection).toHaveBeenCalledWith('col-1')
    })
  })

  it('does not delete a non-empty collection when the user declines', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    collectionsStore.tree = [nonEmptyCollection()]

    const {container} = render(CollectionsTree)

    const deleteBtn = container.querySelectorAll<HTMLButtonElement>('[title="Delete"]')[0]
    await fireEvent.click(deleteBtn)

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(mockBackend.deleteCollection).not.toHaveBeenCalled()
  })
})

describe('HistoryList clear confirmation (Req 7.7)', () => {
  it('clears history only after the user confirms', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    historyStore.entries = [historyEntry()]

    const {container} = render(HistoryList)

    await fireEvent.click(buttonByText(container, 'Clear'))

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    await waitFor(() => {
      expect(mockBackend.clearHistory).toHaveBeenCalledTimes(1)
    })
  })

  it('does not clear history when the user declines', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    historyStore.entries = [historyEntry()]

    const {container} = render(HistoryList)

    await fireEvent.click(buttonByText(container, 'Clear'))

    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(mockBackend.clearHistory).not.toHaveBeenCalled()
  })
})

describe('EnvironmentManager failing-store error path (Req 11.7, 5.9, 7.9)', () => {
  it('surfaces the error and preserves the draft input when the save fails', async () => {
    // The Backend save rejects, so environmentsStore routes the failure into
    // uiStore.errorMessage and the component must keep the open draft.
    mockBackend.saveEnvironment.mockRejectedValue(new Error('disk full'))

    const {container} = render(EnvironmentManager)

    // Open a new-environment draft and type a valid, unique name.
    await fireEvent.click(buttonByText(container, '+ New'))
    const nameInput = container.querySelector<HTMLInputElement>('#env-name')
    expect(nameInput).not.toBeNull()
    await fireEvent.input(nameInput as HTMLInputElement, {target: {value: 'Production'}})

    // Attempt to save; the rejecting Backend drives the error branch.
    await fireEvent.click(buttonByText(container, 'Save'))

    await waitFor(() => {
      expect(uiStore.errorMessage).toBe('disk full')
    })

    // The Backend was asked to save, but the reload never ran (op rejected).
    expect(mockBackend.saveEnvironment).toHaveBeenCalledTimes(1)

    // The draft editor is still open and the user's input is preserved.
    const preserved = container.querySelector<HTMLInputElement>('#env-name')
    expect(preserved).not.toBeNull()
    expect(preserved?.value).toBe('Production')
  })
})
