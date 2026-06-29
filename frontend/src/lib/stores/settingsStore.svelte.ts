// settingsStore mirrors the Go store's Settings and applies the selected theme
// to the document so all views update live (Requirement 9.2) without a restart.
// Settings themselves are persisted by the Backend (Requirement 9.7); this
// store loads them on startup, saves changes through the Backend, and reflects
// the result in `$state`. Theme is applied immediately on load and save.

import { getBackend } from '../backend'
import { defaultSettings, type Settings, type Theme } from '../models'
import { uiStore } from './uiStore.svelte'

const MIN_TIMEOUT = 1
const MAX_TIMEOUT = 600

class SettingsStore {
  /** Current settings; starts at the documented defaults until load() runs. */
  settings = $state<Settings>(defaultSettings())

  /** True while settings are being loaded. */
  loading = $state(false)

  /** Load settings from the Backend and apply the theme. */
  async load(): Promise<void> {
    this.loading = true
    uiStore.clearError()
    try {
      this.settings = await getBackend().getSettings()
    } catch (err) {
      uiStore.showError(err)
    } finally {
      this.loading = false
    }
    applyTheme(this.settings.theme)
  }

  /**
   * Persist the given settings through the Backend, reflect them in state, and
   * apply the theme. On failure the previous settings remain in place.
   */
  async save(next: Settings): Promise<void> {
    uiStore.clearError()
    try {
      await getBackend().saveSettings(next)
      this.settings = next
      applyTheme(this.settings.theme)
    } catch (err) {
      uiStore.showError(err)
    }
  }

  setTheme(theme: Theme): Promise<void> {
    return this.save({ ...this.settings, theme })
  }

  setTlsVerify(tlsVerify: boolean): Promise<void> {
    return this.save({ ...this.settings, tlsVerify })
  }

  /**
   * Update the request timeout. Values outside 1..600 seconds are rejected and
   * the previous timeout is retained, surfacing an error (Requirement 9.6).
   */
  setTimeout(timeoutSeconds: number): Promise<void> {
    if (
      !Number.isInteger(timeoutSeconds) ||
      timeoutSeconds < MIN_TIMEOUT ||
      timeoutSeconds > MAX_TIMEOUT
    ) {
      uiStore.showError(`Timeout must be an integer between ${MIN_TIMEOUT} and ${MAX_TIMEOUT} seconds`)
      return Promise.resolve()
    }
    return this.save({ ...this.settings, timeoutSeconds })
  }
}

/** Tracks the system-theme listener so 'system' mode reacts to OS changes. */
let mediaQuery: MediaQueryList | null = null
let mediaListener: ((e: MediaQueryListEvent) => void) | null = null

/**
 * Apply a theme to the document root by setting `data-theme` to the resolved
 * 'light' or 'dark' value. For 'system' the OS preference is resolved via
 * matchMedia and a listener keeps it in sync while system mode is active.
 */
function applyTheme(theme: Theme): void {
  if (typeof document === 'undefined') return

  // Tear down any previous system listener; only 'system' needs one.
  if (mediaQuery != null && mediaListener != null) {
    mediaQuery.removeEventListener('change', mediaListener)
    mediaQuery = null
    mediaListener = null
  }

  if (theme === 'system') {
    const mq =
      typeof window !== 'undefined' && typeof window.matchMedia === 'function'
        ? window.matchMedia('(prefers-color-scheme: dark)')
        : null
    setResolvedTheme(mq?.matches ? 'dark' : 'light')
    if (mq != null) {
      mediaListener = (e) => setResolvedTheme(e.matches ? 'dark' : 'light')
      mq.addEventListener('change', mediaListener)
      mediaQuery = mq
    }
    return
  }

  setResolvedTheme(theme)
}

function setResolvedTheme(resolved: 'light' | 'dark'): void {
  document.documentElement.setAttribute('data-theme', resolved)
}

export const settingsStore = new SettingsStore()
