<script lang="ts">
  // HistoryList renders the persistent request history. Entries arrive from the
  // Backend already in reverse chronological order (newest first), so this
  // component renders historyStore.entries as-is (Req 7.3). Selecting an entry
  // restores its full request configuration into the editor through
  // requestStore.loadConfig (Req 7.4). Clearing history prompts for
  // confirmation and only clears when the user confirms (Req 7.7); a failing
  // clear surfaces through uiStore.errorMessage without losing the list.

  import { historyStore } from '../../lib/stores/historyStore.svelte'
  import { requestStore } from '../../lib/stores/requestStore.svelte'
  import type { HistoryEntry } from '../../lib/models'

  const entries = $derived(historyStore.entries)

  function selectEntry(entry: HistoryEntry): void {
    requestStore.loadConfig(entry) // Req 7.4 — restore method/url/params/headers/body/auth
  }

  async function clearHistory(): Promise<void> {
    if (entries.length === 0) return
    const confirmed = window.confirm('Clear all request history? This cannot be undone.')
    if (!confirmed) return // declined: leave history unchanged (Req 7.7)
    await historyStore.clear()
  }

  /** Color the status code by its class (2xx/3xx/4xx/5xx) per the token set. */
  function statusColor(status: number): string {
    if (status >= 200 && status < 300) return 'var(--status-success)'
    if (status >= 300 && status < 400) return 'var(--status-redirect)'
    if (status >= 400 && status < 500) return 'var(--status-client-error)'
    if (status >= 500 && status < 600) return 'var(--status-server-error)'
    return 'var(--text-dim)'
  }

  function formatTime(at: number): string {
    try {
      return new Date(at).toLocaleString()
    } catch {
      return ''
    }
  }
</script>

<div class="history">
  <div class="head">
    <span class="title">History</span>
    {#if entries.length > 0}
      <button class="mini" onclick={clearHistory} title="Clear all history">Clear</button>
    {/if}
  </div>

  {#if entries.length === 0}
    <div class="empty">
      <p>No history yet.</p>
      <p class="dim">Sent requests show up here.</p>
    </div>
  {:else}
    <div class="list">
      {#each entries as entry (entry.id)}
        <button class="entry" onclick={() => selectEntry(entry)} title={entry.url}>
          <div class="line">
            <span class="method method-{entry.method.toLowerCase()}">{entry.method}</span>
            {#if entry.error}
              <span class="status error" title={entry.error}>ERR</span>
            {:else}
              <span class="status" style="color: {statusColor(entry.status)}">{entry.status || '—'}</span>
            {/if}
          </div>
          <div class="url">{entry.url}</div>
          <div class="meta">
            <span class="time">{formatTime(entry.at)}</span>
            {#if !entry.error}<span class="dur">{entry.durationMs} ms</span>{/if}
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .history {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: var(--space-4);
    gap: var(--space-2);
    overflow: hidden;
  }

  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .title {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--text-dim);
  }

  .mini {
    background: var(--bg-elev-2);
    border: 1px solid var(--border);
    color: var(--text-dim);
    font-size: var(--font-size-xs);
    padding: var(--space-1) var(--space-2);
    cursor: pointer;
  }

  .mini:hover {
    color: var(--text);
  }

  .list {
    flex: 1;
    overflow: auto;
    display: flex;
    flex-direction: column;
  }

  .entry {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    border-bottom: 1px solid var(--border);
    padding: var(--space-2) 0;
    cursor: pointer;
  }

  .entry:hover {
    background: var(--bg-elev-2);
  }

  .line {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .method {
    font-weight: var(--font-weight-bold);
    font-size: var(--font-size-xs);
  }

  .method-get { color: var(--green); }
  .method-post { color: var(--accent); }
  .method-put { color: var(--blue); }
  .method-patch { color: var(--purple); }
  .method-delete { color: var(--red); }
  .method-head,
  .method-options { color: var(--text-dim); }

  .status {
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-semibold);
  }

  .status.error {
    color: var(--red);
  }

  .url {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    color: var(--text-dim);
    font-size: var(--font-size-xs);
  }

  .empty {
    color: var(--text-dim);
    font-size: var(--font-size-sm);
    padding: var(--space-4) 0;
    text-align: center;
  }

  .empty p {
    margin: 0;
  }

  .empty .dim {
    opacity: 0.7;
    font-size: var(--font-size-xs);
  }

  .dim {
    color: var(--text-dim);
  }
</style>
