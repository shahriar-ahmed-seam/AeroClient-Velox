// uiStore holds UI-only view state: which top-level view and config panels are
// active, whether the sidebar is collapsed, whether the command palette is
// open, the active response-panel tab, and the transient error/toast messages.
//
// This store deliberately holds NO persistent data and talks to no Backend —
// it is the surface other stores use to surface failures (showError) without
// crashing, per the Error Handling section of the design.

/** Top-level view shown in the workbench area. */
export type ActiveView = 'workbench' | 'settings'

/** The request-configuration tab currently shown in the editor. */
export type ConfigTab = 'params' | 'body' | 'headers' | 'auth'

/** The tab currently shown in the response viewer. */
export type ResponseTab = 'body' | 'headers'

class UIStore {
  /** Which top-level view is visible. */
  activeView = $state<ActiveView>('workbench')

  /** Active request-configuration tab in the editor. */
  activeConfigTab = $state<ConfigTab>('params')

  /** Active tab in the response (right) panel. */
  activeResponseTab = $state<ResponseTab>('body')

  /** Whether the sidebar is collapsed (hidden) on narrow layouts. */
  sidebarCollapsed = $state(false)

  /** Whether the command palette overlay is open. */
  commandPaletteOpen = $state(false)

  /**
   * The most recent error to surface to the user, or null when there is none.
   * Stores route Backend failures here instead of throwing, so the UI can show
   * an indication while preserving the user's current input.
   */
  errorMessage = $state<string | null>(null)

  /** A transient, non-error confirmation message (e.g. "Copied"). */
  toastMessage = $state<string | null>(null)

  setActiveView(view: ActiveView): void {
    this.activeView = view
  }

  setConfigTab(tab: ConfigTab): void {
    this.activeConfigTab = tab
  }

  setResponseTab(tab: ResponseTab): void {
    this.activeResponseTab = tab
  }

  toggleSidebar(): void {
    this.sidebarCollapsed = !this.sidebarCollapsed
  }

  setSidebarCollapsed(collapsed: boolean): void {
    this.sidebarCollapsed = collapsed
  }

  openCommandPalette(): void {
    this.commandPaletteOpen = true
  }

  closeCommandPalette(): void {
    this.commandPaletteOpen = false
  }

  toggleCommandPalette(): void {
    this.commandPaletteOpen = !this.commandPaletteOpen
  }

  /** Surface an error to the user. Accepts an Error or any thrown value. */
  showError(err: unknown): void {
    this.errorMessage = messageOf(err)
  }

  clearError(): void {
    this.errorMessage = null
  }

  showToast(message: string): void {
    this.toastMessage = message
  }

  clearToast(): void {
    this.toastMessage = null
  }
}

/** Normalizes any thrown value into a human-readable message string. */
function messageOf(err: unknown): string {
  if (err instanceof Error) return err.message
  if (typeof err === 'string') return err
  try {
    return String(err)
  } catch {
    return 'An unknown error occurred'
  }
}

export const uiStore = new UIStore()
