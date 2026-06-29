// environmentsStore mirrors the Go store's Environments. The list is held in
// `$state` and the active environment is a `$derived` projection of it (the Go
// store guarantees at most one active environment). Every mutation routes
// through the Backend and then reloads the list so the active flag and variable
// edits always reflect persisted state (Requirements 6.1-6.3, 6.7, 6.10).

import { getBackend } from '../backend'
import type { Environment } from '../models'
import { uiStore } from './uiStore.svelte'

class EnvironmentsStore {
  /** All environments as returned by the Backend. */
  environments = $state<Environment[]>([])

  /** True while environments are being (re)loaded. */
  loading = $state(false)

  /**
   * The currently active environment, or null when none is active. Derived from
   * the list, so it updates automatically whenever environments are reloaded.
   * This is what interpolationStore reads to resolve {{tokens}}.
   */
  activeEnvironment = $derived<Environment | null>(
    this.environments.find((e) => e.active) ?? null,
  )

  async load(): Promise<void> {
    this.loading = true
    uiStore.clearError()
    try {
      this.environments = await getBackend().listEnvironments()
    } catch (err) {
      uiStore.showError(err)
    } finally {
      this.loading = false
    }
  }

  async save(env: Environment): Promise<void> {
    await this.mutate(() => getBackend().saveEnvironment(env))
  }

  async delete(id: string): Promise<void> {
    await this.mutate(() => getBackend().deleteEnvironment(id))
  }

  /** Activate an environment, or pass "" to clear the active selection. */
  async setActive(id: string): Promise<void> {
    await this.mutate(() => getBackend().setActiveEnvironment(id))
  }

  private async mutate(op: () => Promise<unknown>): Promise<void> {
    uiStore.clearError()
    try {
      await op()
      this.environments = await getBackend().listEnvironments()
    } catch (err) {
      uiStore.showError(err)
    }
  }
}

export const environmentsStore = new EnvironmentsStore()
