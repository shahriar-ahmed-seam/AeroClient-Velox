<script lang="ts">
  // HeadersView lists the response headers as name/value pairs in the exact
  // order the engine returned them. The engine preserves received order in the
  // HTTPResponse.headers slice, so we iterate it as-is without sorting.
  //
  // Requirements: 4.8 (display all headers preserving received order).
  import type { KeyValue } from '../../lib/models'

  let { headers }: { headers: KeyValue[] } = $props()
</script>

<div class="headers">
  {#if headers.length === 0}
    <div class="empty">No response headers.</div>
  {:else}
    {#each headers as header, i (i)}
      <div class="row">
        <span class="hk">{header.key}</span>
        <span class="hv">{header.value}</span>
      </div>
    {/each}
  {/if}
</div>

<style>
  .headers {
    flex: 1;
    overflow: auto;
    padding: var(--space-2) var(--space-4);
  }
  .empty {
    color: var(--text-dim);
    font-size: var(--font-size-sm);
    padding: var(--space-4) 0;
  }
  .row {
    display: grid;
    grid-template-columns: minmax(160px, 280px) 1fr;
    gap: var(--space-3);
    padding: var(--space-1) 0;
    border-bottom: 1px solid var(--border);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    user-select: text;
  }
  .hk {
    color: var(--accent-2);
    word-break: break-all;
  }
  .hv {
    color: var(--text);
    word-break: break-all;
    white-space: pre-wrap;
  }
</style>
