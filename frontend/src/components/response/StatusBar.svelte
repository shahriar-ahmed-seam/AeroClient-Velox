<script lang="ts">
  // StatusBar renders the headline metrics of a completed response: the numeric
  // status code, its status text, the elapsed time in whole milliseconds, and
  // the body size in bytes. The status code/text are tinted by status class
  // (2xx/3xx/4xx/5xx) using the shared statusColor token mapping.
  //
  // Requirements: 4.1 (status code, text, elapsed ms, size bytes),
  // 4.2 (status-class color).
  import { statusColor, formatBytes } from '../../lib/types'
  import type { HTTPResponse } from '../../lib/models'

  let { response }: { response: HTTPResponse } = $props()

  // The engine returns statusText that may already embed the code (e.g.
  // "200 OK"); strip a leading copy of the code so we never render it twice.
  const statusText = $derived(
    response.statusText.replace(String(response.status), '').trim(),
  )
  const color = $derived(statusColor(response.status))
</script>

<div class="statusbar">
  <span class="status" style="color:{color}">
    <span class="code">{response.status}</span>
    {#if statusText}<span class="text">{statusText}</span>{/if}
  </span>
  <span class="metric" title="Elapsed time">
    <span class="metric-label">Time</span>
    <span class="metric-value">{response.durationMs} ms</span>
  </span>
  <span class="metric" title="Response body size">
    <span class="metric-label">Size</span>
    <span class="metric-value">{formatBytes(response.sizeBytes)}</span>
  </span>
</div>

<style>
  .statusbar {
    display: flex;
    align-items: center;
    gap: var(--space-6);
    padding: var(--space-2) var(--space-4);
    border-bottom: 1px solid var(--border);
    font-size: var(--font-size-sm);
  }
  .status {
    font-weight: var(--font-weight-bold);
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
  }
  .code { font-size: var(--font-size-md); }
  .text { font-weight: var(--font-weight-medium); }
  .metric {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    color: var(--text-dim);
  }
  .metric-label {
    text-transform: uppercase;
    letter-spacing: 0.5px;
    font-size: var(--font-size-xs);
  }
  .metric-value {
    color: var(--text);
    font-family: var(--font-mono);
  }
</style>
