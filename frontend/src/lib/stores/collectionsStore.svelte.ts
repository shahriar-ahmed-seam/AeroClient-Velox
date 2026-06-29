// collectionsStore mirrors the Go store's collection/folder/request tree. The
// tree itself is the single source of truth held in `$state`; every mutation is
// routed through the Backend (getBackend()) and then the tree is refreshed from
// the Backend's ListTree so the in-memory view always matches persisted state
// (Requirement 5.5). Failures are surfaced through uiStore without throwing.

import { getBackend } from '../backend'
import type { Collection, Folder, SavedRequest } from '../models'
import { uiStore } from './uiStore.svelte'

class CollectionsStore {
  /** The full collection tree (collections -> folders/requests). */
  tree = $state<Collection[]>([])

  /** True while the tree is being (re)loaded. */
  loading = $state(false)

  /** Reload the entire tree from the Backend. */
  async load(): Promise<void> {
    this.loading = true
    uiStore.clearError()
    try {
      this.tree = await getBackend().listTree()
    } catch (err) {
      uiStore.showError(err)
    } finally {
      this.loading = false
    }
  }

  async saveCollection(c: Collection): Promise<void> {
    await this.mutate(() => getBackend().saveCollection(c))
  }

  async renameCollection(id: string, name: string): Promise<void> {
    await this.mutate(() => getBackend().renameCollection(id, name))
  }

  async deleteCollection(id: string): Promise<void> {
    await this.mutate(() => getBackend().deleteCollection(id))
  }

  async saveFolder(folder: Folder, parentId: string): Promise<void> {
    await this.mutate(() => getBackend().saveFolder(folder, parentId))
  }

  async deleteFolder(id: string): Promise<void> {
    await this.mutate(() => getBackend().deleteFolder(id))
  }

  async saveRequest(req: SavedRequest, parentId: string): Promise<void> {
    await this.mutate(() => getBackend().saveRequest(req, parentId))
  }

  async deleteRequest(id: string): Promise<void> {
    await this.mutate(() => getBackend().deleteRequest(id))
  }

  async moveRequest(requestId: string, targetParentId: string): Promise<void> {
    await this.mutate(() => getBackend().moveRequest(requestId, targetParentId))
  }

  /**
   * Run a Backend mutation then refresh the tree. On failure the error is
   * surfaced via uiStore and the previously loaded tree is left untouched so a
   * failed save never discards the user's view (design Error Handling).
   */
  private async mutate(op: () => Promise<unknown>): Promise<void> {
    uiStore.clearError()
    try {
      await op()
      this.tree = await getBackend().listTree()
    } catch (err) {
      uiStore.showError(err)
    }
  }
}

export const collectionsStore = new CollectionsStore()
