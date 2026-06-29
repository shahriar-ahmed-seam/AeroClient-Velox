<script lang="ts">
  // CommandPalette — keyboard-invoked overlay for searching and running commands
  // (Requirement 10.3, 10.4, 10.7, 10.8, 10.9).
  //
  // Open state is owned by uiStore.commandPaletteOpen (toggled by Ctrl/Cmd+K via
  // the global shortcut handler). Typing filters the registry with the pure
  // filterCommands helper; Up/Down move the highlight, Enter runs the highlighted
  // command, pointer click runs the clicked command, and Escape closes without
  // running anything. When the filter matches nothing a no-results indication is
  // shown and no command can be executed.

  import { createCommands, filterCommands, type Command } from '../../lib/commands'
  import { requestStore } from '../../lib/stores/requestStore.svelte'
  import { uiStore } from '../../lib/stores/uiStore.svelte'

  // The default command registry, wired to the real stores here so commands.ts
  // stays pure and store-free.
  const commands: Command[] = createCommands({
    sendRequest: () => void requestStore.send(),
    saveRequest: () => {
      // The editor save flow (choosing a collection) lives in the sidebar UI;
      // this is a no-op hook until that action is exposed.
    },
    openSettings: () => uiStore.setActiveView('settings'),
    openWorkbench: () => uiStore.setActiveView('workbench'),
  })

  let query = $state('')
  let selectedIndex = $state(0)
  let inputEl = $state<HTMLInputElement | null>(null)

  const open = $derived(uiStore.commandPaletteOpen)
  const filtered = $derived(filterCommands(commands, query))

  // Reset the query and highlight each time the palette opens, and move focus to
  // the search input so the user can type immediately.
  $effect(() => {
    if (open) {
      query = ''
      selectedIndex = 0
      // Focus after the input is rendered.
      queueMicrotask(() => inputEl?.focus())
    }
  })

  // Keep the highlight within the bounds of the current results as they change.
  $effect(() => {
    const max = Math.max(0, filtered.length - 1)
    if (selectedIndex > max) selectedIndex = max
  })

  function close(): void {
    uiStore.closeCommandPalette()
  }

  function runCommand(command: Command): void {
    // Close first so a command that changes view leaves the palette dismissed.
    close()
    command.run()
  }

  function moveHighlight(delta: number): void {
    if (filtered.length === 0) return
    const count = filtered.length
    selectedIndex = (selectedIndex + delta + count) % count
  }

  function onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      event.preventDefault()
      close()
      return
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      moveHighlight(1)
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      moveHighlight(-1)
      return
    }
    if (event.key === 'Enter') {
      event.preventDefault()
      const command = filtered[selectedIndex]
      if (command) runCommand(command)
    }
  }
</script>

{#if open}
  <!-- Backdrop: clicking outside the panel closes the palette. -->
  <div
    class="overlay"
    role="presentation"
    onclick={close}
    onkeydown={onKeydown}
  >
    <!-- Panel: stop propagation so clicks inside don't dismiss; keydown drives nav. -->
    <div
      class="panel"
      role="dialog"
      aria-modal="true"
      aria-label="Command palette"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
      onkeydown={onKeydown}
    >
      <input
        bind:this={inputEl}
        bind:value={query}
        class="search"
        type="text"
        placeholder="Type a command…"
        spellcheck="false"
        aria-label="Search commands"
      />

      {#if filtered.length === 0}
        <div class="no-results">No matching commands</div>
      {:else}
        <ul class="results" role="listbox">
          {#each filtered as command, i (command.id)}
            <li role="option" aria-selected={i === selectedIndex}>
              <button
                type="button"
                class="result"
                class:active={i === selectedIndex}
                onmousemove={() => (selectedIndex = i)}
                onclick={() => runCommand(command)}
              >
                {command.title}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding-top: var(--space-12);
    z-index: 1000;
  }

  .panel {
    width: 100%;
    max-width: 560px;
    background: var(--bg-elev);
    border: 1px solid var(--border-strong);
    display: flex;
    flex-direction: column;
    max-height: 60vh;
    overflow: hidden;
  }

  .search {
    background: var(--bg);
    border: none;
    border-bottom: 1px solid var(--border);
    color: var(--text);
    font-family: var(--font-sans);
    font-size: var(--font-size-md);
    padding: var(--space-4);
    outline: none;
  }
  .search::placeholder {
    color: var(--text-dim);
  }

  .results {
    list-style: none;
    margin: 0;
    padding: var(--space-1);
    overflow-y: auto;
  }

  .result {
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    color: var(--text);
    font-family: var(--font-sans);
    font-size: var(--font-size-md);
    padding: var(--space-3) var(--space-4);
    cursor: pointer;
  }
  .result:hover,
  .result.active {
    background: var(--bg-elev-2);
    color: var(--accent);
  }

  .no-results {
    color: var(--text-dim);
    font-family: var(--font-sans);
    font-size: var(--font-size-md);
    padding: var(--space-4);
  }
</style>
