<script lang="ts">
  // App — the top-level shell that assembles the full Volt UI and implements the
  // responsive layout (Requirement 12).
  //
  // Layout regions (left → right): an always-visible icon rail, the main column
  // (top bar + workbench or settings), and a sidebar that hosts the collections
  // tree, history list, and environment manager. The command palette is rendered
  // unconditionally and shows itself when uiStore.commandPaletteOpen is true.
  //
  // Responsive behavior, driven by a reactive viewport-width tracker:
  //   - Wide   (≥1024px): rail + workbench + sidebar shown simultaneously as a
  //     multi-column layout, no horizontal scrolling (Req 12.1).
  //   - Medium (600–1023px): the sidebar collapses into a toggleable overlay
  //     drawer beside the workbench (Req 12.2).
  //   - Narrow (<600px): the sidebar becomes a bottom sheet and the request
  //     configuration tabs / response tabs present one panel at a time with the
  //     tab strip acting as the navigation control (Req 12.3, 12.4).
  // Crossing a breakpoint reflows the layout and auto-collapses/expands the
  // sidebar so every region remains reachable (Req 12.5). All controls stay
  // within the viewport and usable from 320px to 3840px (Req 12.6).

  import { onMount, onDestroy } from 'svelte'

  import {
    requestStore,
    settingsStore,
    collectionsStore,
    environmentsStore,
    historyStore,
    uiStore,
    type ConfigTab,
  } from './lib/stores'
  import { installGlobalShortcuts } from './lib/shortcuts'

  import UrlBar from './components/request/UrlBar.svelte'
  import ParamsTable from './components/request/ParamsTable.svelte'
  import BodyEditor from './components/request/BodyEditor.svelte'
  import HeadersTable from './components/request/HeadersTable.svelte'
  import AuthPanel from './components/request/AuthPanel.svelte'
  import ResponseViewer from './components/response/ResponseViewer.svelte'
  import CollectionsTree from './components/sidebar/CollectionsTree.svelte'
  import HistoryList from './components/sidebar/HistoryList.svelte'
  import EnvironmentManager from './components/sidebar/EnvironmentManager.svelte'
  import CommandPalette from './components/command/CommandPalette.svelte'
  import SettingsView from './components/settings/SettingsView.svelte'

  // --- responsive width tracking -------------------------------------------

  let innerWidth = $state(typeof window !== 'undefined' ? window.innerWidth : 1280)

  type Breakpoint = 'wide' | 'medium' | 'narrow'
  const breakpoint = $derived<Breakpoint>(
    innerWidth >= 1024 ? 'wide' : innerWidth >= 600 ? 'medium' : 'narrow',
  )

  /** On medium/narrow the sidebar floats over the workbench as a drawer/sheet. */
  const sidebarIsOverlay = $derived(breakpoint !== 'wide')

  // Reflow when a breakpoint boundary is crossed: collapse the sidebar on
  // medium/narrow, expand it on wide. This effect reads only `breakpoint`, so it
  // re-runs solely when the breakpoint changes — user toggles in between are not
  // overridden (Req 12.5).
  $effect(() => {
    uiStore.setSidebarCollapsed(breakpoint !== 'wide')
  })

  // --- sidebar section -------------------------------------------------------

  type SidebarSection = 'collections' | 'history' | 'environments'
  let sidebarSection = $state<SidebarSection>('collections')

  const sidebarOpen = $derived(!uiStore.sidebarCollapsed)

  function openSection(section: SidebarSection): void {
    sidebarSection = section
    uiStore.setActiveView('workbench')
    uiStore.setSidebarCollapsed(false)
  }

  function toggleSidebar(): void {
    uiStore.toggleSidebar()
  }

  function closeSidebar(): void {
    uiStore.setSidebarCollapsed(true)
  }

  // --- request configuration tabs -------------------------------------------

  const configTabs: { id: ConfigTab; label: string }[] = [
    { id: 'params', label: 'Parameters' },
    { id: 'body', label: 'Body' },
    { id: 'headers', label: 'Headers' },
    { id: 'auth', label: 'Authorization' },
  ]

  const activeConfigTab = $derived(uiStore.activeConfigTab)

  // Live counts surfaced as badges on the tab strip.
  const activeParamCount = $derived(
    requestStore.current.params.filter((p) => p.enabled && p.key.trim() !== '').length,
  )
  const activeHeaderCount = $derived(
    requestStore.current.headers.filter((h) => h.enabled && h.key.trim() !== '').length,
  )

  // --- startup / teardown ----------------------------------------------------

  let uninstallShortcuts: (() => void) | null = null

  onMount(() => {
    // Load persisted state from the Backend. Each store routes failures through
    // uiStore, so these are fire-and-forget.
    void settingsStore.load()
    void collectionsStore.load()
    void environmentsStore.load()
    void historyStore.load()

    uninstallShortcuts = installGlobalShortcuts()
  })

  onDestroy(() => {
    uninstallShortcuts?.()
    uninstallShortcuts = null
  })
</script>

<svelte:window bind:innerWidth />

<div class="app" data-breakpoint={breakpoint}>
  <!-- Icon rail: always visible, switches sidebar section / view (Req 12.1). -->
  <nav class="rail" aria-label="Primary">
    <div class="rail-logo" title="Volt">⚡</div>

    <button
      class="rail-btn"
      class:active={uiStore.activeView === 'workbench' && sidebarSection === 'collections'}
      title="Collections"
      aria-label="Collections"
      onclick={() => openSection('collections')}
    >▤</button>
    <button
      class="rail-btn"
      class:active={uiStore.activeView === 'workbench' && sidebarSection === 'history'}
      title="History"
      aria-label="History"
      onclick={() => openSection('history')}
    >◔</button>
    <button
      class="rail-btn"
      class:active={uiStore.activeView === 'workbench' && sidebarSection === 'environments'}
      title="Environments"
      aria-label="Environments"
      onclick={() => openSection('environments')}
    >◍</button>

    <div class="rail-spacer"></div>

    <button
      class="rail-btn"
      title="Command palette"
      aria-label="Command palette"
      onclick={() => uiStore.openCommandPalette()}
    >⌘</button>
    <button
      class="rail-btn"
      class:active={uiStore.activeView === 'settings'}
      title="Settings"
      aria-label="Settings"
      onclick={() => uiStore.setActiveView('settings')}
    >⚙</button>
  </nav>

  <div class="main">
    <!-- Top bar -->
    <header class="topbar">
      {#if uiStore.activeView === 'workbench'}
        <button
          class="topbar-toggle"
          title={sidebarOpen ? 'Hide sidebar' : 'Show sidebar'}
          aria-label={sidebarOpen ? 'Hide sidebar' : 'Show sidebar'}
          aria-expanded={sidebarOpen}
          onclick={toggleSidebar}
        >☰</button>
      {/if}

      <div class="brand">VOLT <span class="brand-dim">api client</span></div>

      <button
        class="cmd"
        title="Open command palette"
        aria-label="Open command palette"
        onclick={() => uiStore.openCommandPalette()}
      >
        <span class="cmd-icon" aria-hidden="true">⌕</span>
        <span class="cmd-text">Search and commands</span>
        <span class="cmd-kbd">Ctrl K</span>
      </button>
    </header>

    {#if uiStore.activeView === 'settings'}
      <div class="settings-scroll">
        <SettingsView />
      </div>
    {:else}
      <div class="body">
        <!-- Sidebar: inline column on wide, overlay drawer/sheet otherwise. -->
        {#if sidebarOpen && sidebarIsOverlay}
          <button
            class="backdrop"
            aria-label="Close sidebar"
            onclick={closeSidebar}
          ></button>
        {/if}

        <aside
          class="sidebar"
          class:overlay={sidebarIsOverlay}
          class:sheet={breakpoint === 'narrow'}
          class:open={sidebarOpen}
          aria-hidden={!sidebarOpen}
        >
          <div class="side-tabs" role="tablist" aria-label="Sidebar sections">
            <button
              class="side-tab"
              class:active={sidebarSection === 'collections'}
              role="tab"
              aria-selected={sidebarSection === 'collections'}
              onclick={() => (sidebarSection = 'collections')}
            >Collections</button>
            <button
              class="side-tab"
              class:active={sidebarSection === 'history'}
              role="tab"
              aria-selected={sidebarSection === 'history'}
              onclick={() => (sidebarSection = 'history')}
            >History</button>
            <button
              class="side-tab"
              class:active={sidebarSection === 'environments'}
              role="tab"
              aria-selected={sidebarSection === 'environments'}
              onclick={() => (sidebarSection = 'environments')}
            >Env</button>

            {#if sidebarIsOverlay}
              <button class="side-close" title="Close" aria-label="Close sidebar" onclick={closeSidebar}>✕</button>
            {/if}
          </div>

          <div class="side-content">
            {#if sidebarSection === 'collections'}
              <CollectionsTree />
            {:else if sidebarSection === 'history'}
              <HistoryList />
            {:else}
              <EnvironmentManager />
            {/if}
          </div>
        </aside>

        <!-- Workbench: request editor (top) + response viewer (fills below). -->
        <section class="workbench" aria-label="Request workbench">
          <div class="editor">
            <UrlBar />

            <div class="cfgtabs" role="tablist" aria-label="Request configuration">
              {#each configTabs as tab (tab.id)}
                <button
                  class="cfgtab"
                  class:active={activeConfigTab === tab.id}
                  role="tab"
                  aria-selected={activeConfigTab === tab.id}
                  onclick={() => uiStore.setConfigTab(tab.id)}
                >
                  {tab.label}
                  {#if tab.id === 'params' && activeParamCount > 0}
                    <span class="badge">{activeParamCount}</span>
                  {:else if tab.id === 'headers' && activeHeaderCount > 0}
                    <span class="badge">{activeHeaderCount}</span>
                  {/if}
                </button>
              {/each}
            </div>

            <div class="cfgpanel">
              {#if activeConfigTab === 'params'}
                <ParamsTable />
              {:else if activeConfigTab === 'body'}
                <BodyEditor />
              {:else if activeConfigTab === 'headers'}
                <HeadersTable />
              {:else}
                <AuthPanel />
              {/if}
            </div>
          </div>

          <div class="response-pane">
            <ResponseViewer />
          </div>
        </section>
      </div>
    {/if}
  </div>
</div>

<!-- Command palette: always mounted, shows itself when open (Req 10.2). -->
<CommandPalette />

<!-- Error / toast surfaces (Req 11.7). -->
{#if uiStore.errorMessage}
  <div class="banner error" role="alert">
    <span class="banner-text">{uiStore.errorMessage}</span>
    <button class="banner-close" aria-label="Dismiss error" onclick={() => uiStore.clearError()}>✕</button>
  </div>
{/if}

{#if uiStore.toastMessage}
  <div class="toast" role="status" aria-live="polite">
    <span class="toast-text">{uiStore.toastMessage}</span>
    <button class="toast-close" aria-label="Dismiss" onclick={() => uiStore.clearToast()}>✕</button>
  </div>
{/if}

<style>
  .app {
    display: flex;
    height: 100vh;
    overflow: hidden;
  }

  /* --- icon rail ---------------------------------------------------------- */
  .rail {
    flex: 0 0 auto;
    width: 48px;
    background: var(--bg-elev);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: var(--space-2) 0;
    gap: var(--space-2);
  }
  .rail-logo {
    font-size: var(--font-size-xl);
    color: var(--accent);
    margin-bottom: var(--space-2);
  }
  .rail-btn {
    width: 34px;
    height: 34px;
    border: none;
    background: transparent;
    color: var(--text-dim);
    cursor: pointer;
    font-size: var(--font-size-lg);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .rail-btn:hover {
    background: var(--bg-elev-2);
    color: var(--text);
  }
  .rail-btn.active {
    color: var(--accent);
  }
  .rail-spacer {
    flex: 1;
  }

  /* --- main column -------------------------------------------------------- */
  .main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }

  .topbar {
    flex: 0 0 auto;
    height: 48px;
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    gap: var(--space-4);
    padding: 0 var(--space-4);
    background: var(--bg-elev);
  }
  .topbar-toggle {
    background: transparent;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    font-size: var(--font-size-lg);
    padding: var(--space-1) var(--space-2);
  }
  .topbar-toggle:hover {
    color: var(--text);
  }
  .brand {
    font-weight: var(--font-weight-bold);
    letter-spacing: 1px;
    font-size: var(--font-size-md);
    color: var(--accent);
    white-space: nowrap;
  }
  .brand-dim {
    color: var(--text-dim);
    font-weight: var(--font-weight-regular);
    letter-spacing: 0;
    font-size: var(--font-size-xs);
  }
  .cmd {
    flex: 1;
    max-width: 460px;
    margin: 0 auto;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    background: var(--bg);
    border: 1px solid var(--border);
    padding: var(--space-2) var(--space-3);
    cursor: pointer;
    color: var(--text-dim);
    min-width: 0;
  }
  .cmd:hover {
    border-color: var(--border-strong);
  }
  .cmd-icon {
    color: var(--text-dim);
  }
  .cmd-text {
    flex: 1;
    text-align: left;
    font-size: var(--font-size-sm);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .cmd-kbd {
    color: var(--text-dim);
    font-size: var(--font-size-xs);
    border: 1px solid var(--border);
    padding: 1px var(--space-2);
    white-space: nowrap;
  }

  /* --- body: sidebar + workbench ----------------------------------------- */
  .body {
    flex: 1;
    display: flex;
    min-height: 0;
    position: relative;
  }

  .sidebar {
    flex: 0 0 280px;
    width: 280px;
    border-right: 1px solid var(--border);
    background: var(--bg-elev);
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  /* Hidden inline sidebar (wide layout, user-collapsed). */
  .sidebar:not(.open):not(.overlay) {
    display: none;
  }
  /* Overlay drawer for medium / narrow. */
  .sidebar.overlay {
    position: absolute;
    top: 0;
    left: 0;
    bottom: 0;
    z-index: 30;
    width: min(320px, 85vw);
    flex-basis: auto;
    border-right: 1px solid var(--border-strong);
    box-shadow: 2px 0 12px rgba(0, 0, 0, 0.4);
  }
  .sidebar.overlay:not(.open) {
    display: none;
  }
  /* Bottom sheet for narrow viewports (Req 12.3). */
  .sidebar.sheet {
    top: auto;
    left: 0;
    right: 0;
    bottom: 0;
    width: 100%;
    max-height: 70vh;
    border-right: none;
    border-top: 1px solid var(--border-strong);
    box-shadow: 0 -2px 12px rgba(0, 0, 0, 0.4);
  }

  .backdrop {
    position: absolute;
    inset: 0;
    z-index: 20;
    background: rgba(0, 0, 0, 0.45);
    border: none;
    padding: 0;
    cursor: pointer;
  }

  .side-tabs {
    flex: 0 0 auto;
    display: flex;
    align-items: stretch;
    gap: var(--space-1);
    padding: 0 var(--space-2);
    border-bottom: 1px solid var(--border);
    overflow-x: auto;
  }
  .side-tab {
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    color: var(--text-dim);
    padding: var(--space-3) var(--space-2);
    cursor: pointer;
    font-size: var(--font-size-sm);
    white-space: nowrap;
  }
  .side-tab:hover {
    color: var(--text);
  }
  .side-tab.active {
    color: var(--text);
    border-bottom-color: var(--accent);
  }
  .side-close {
    margin-left: auto;
    background: transparent;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    font-size: var(--font-size-sm);
    padding: 0 var(--space-2);
  }
  .side-close:hover {
    color: var(--text);
  }
  .side-content {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  /* --- workbench ---------------------------------------------------------- */
  .workbench {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
  .editor {
    flex: 0 0 auto;
    display: flex;
    flex-direction: column;
    min-height: 0;
    max-height: 55%;
  }
  .cfgtabs {
    flex: 0 0 auto;
    display: flex;
    gap: var(--space-1);
    padding: 0 var(--space-4);
    border-bottom: 1px solid var(--border);
    overflow-x: auto;
  }
  .cfgtab {
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    color: var(--text-dim);
    padding: var(--space-3) var(--space-2);
    cursor: pointer;
    font-size: var(--font-size-md);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    white-space: nowrap;
  }
  .cfgtab:hover {
    color: var(--text);
  }
  .cfgtab.active {
    color: var(--text);
    border-bottom-color: var(--accent);
  }
  .badge {
    background: var(--bg-elev-2);
    color: var(--accent);
    font-size: var(--font-size-xs);
    padding: 0 var(--space-2);
  }
  .cfgpanel {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--space-3) var(--space-4);
  }

  .response-pane {
    flex: 1;
    min-height: 0;
    display: flex;
  }

  /* --- settings ----------------------------------------------------------- */
  .settings-scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }

  /* --- error banner / toast ---------------------------------------------- */
  .banner {
    position: fixed;
    top: var(--space-4);
    left: 50%;
    transform: translateX(-50%);
    z-index: 2000;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    max-width: min(640px, calc(100vw - var(--space-8)));
    padding: var(--space-3) var(--space-4);
    background: var(--bg-elev-2);
    border: 1px solid var(--red);
    color: var(--text);
    font-size: var(--font-size-sm);
  }
  .banner-text {
    flex: 1;
    word-break: break-word;
  }
  .banner-close,
  .toast-close {
    background: transparent;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    font-size: var(--font-size-sm);
    padding: 0 var(--space-1);
  }
  .banner-close:hover,
  .toast-close:hover {
    color: var(--text);
  }

  .toast {
    position: fixed;
    bottom: var(--space-4);
    left: 50%;
    transform: translateX(-50%);
    z-index: 2000;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    max-width: min(560px, calc(100vw - var(--space-8)));
    padding: var(--space-2) var(--space-4);
    background: var(--bg-elev-2);
    border: 1px solid var(--border-strong);
    color: var(--text);
    font-size: var(--font-size-sm);
  }

  /* On narrow viewports the editor splits less aggressively so the response
     stays usable; the workbench scrolls as a whole if space is tight. */
  .app[data-breakpoint='narrow'] .editor {
    max-height: 60%;
  }
</style>
