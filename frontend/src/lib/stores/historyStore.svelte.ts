// historyStore mirrors the Go store's request History. History entries are
// recorded by the Backend during request execution (not by the frontend), so
// this store never fabricates persistence: `add` reconciles by reloading the
// authoritative list from the Backend. Entries are presented newest-first as
// the Backend returns them in reverse chronological order (Requirement 7.3).
// Selecting an entry to restore its configuration is handled by
// requestStore.loadConfig (Requirement 7.4).

import { getBackend } from '../backend'
import type { HistoryEntry } from '../models'
import { uiStore } from './uiStore.svelte'

class HistoryStore {
  /** History entries in reverse chronological order (newest first). */
  entries = $state<HistoryEntry[]>([])

  /** True while history is being (re)loaded. */
  loading = $state(false)

  /** Reload all history entries from the Backend. */
  async load(): Promise<void> {
    this.loading = true
    uiStore.clearError()
    try {
      this.entries = await getBackend().listHistory()
    } catch (err) {
      uiStore.showError(err)
    } finally {
      this.loading = false
    }
  }

  /**
   * Refresh history after the Backend has recorded a new entry. The Backend
   * persists History as part of executing a request, so there is no
   * frontend-side write; this simply pulls the updated list back into state.
   */
  async add(): Promise<void> {
    await this.load()
  }

  /** Clear all history through the Backend, then refresh. */
  async clear(): Promise<void> {
    uiStore.clearError()
    try {
      await getBackend().clearHistory()
      this.entries = await getBackend().listHistory()
    } catch (err) {
      uiStore.showError(err)
    }
  }
}

export const historyStore = new HistoryStore()
