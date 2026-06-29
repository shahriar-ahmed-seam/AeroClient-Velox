<script lang="ts">
  // ResponseViewer is the container for the response panel. It reads the current
  // response and in-flight flag from requestStore and the active response tab
  // from uiStore, then composes StatusBar, BodyView, and HeadersView.
  //
  // Render precedence:
  //   loading            → loading indicator (Req 4.10)
  //   no response yet    → empty state (Req 4.1)
  //   response.error set → error message, no status/time/size (Req 4.9)
  //   otherwise          → StatusBar + Body/Headers tabs (Req 4.1–4.8, 4.11, 4.12)
  import { requestStore, uiStore } from '../../lib/stores'
  import StatusBar from './StatusBar.svelte'
  import BodyView from './BodyView.svelte'
  import HeadersView from './HeadersView.svelte'

  const response = $derived(requestStore.response)
  const loading = $derived(requestStore.loading)
  const activeTab = $derived(uiStore.activeResponseTab)
  const hasError = $derived(!!response && response.error !== '')
</script>

<section class="response" aria-label="Response">
  <div class="head">
    <span class="label">Response</span>
    {#if response && !hasError && !loading}
      <StatusBar {response} />
    {/if}
  </div>

  {#if loading}
    <div class="state" role="status" aria-live="polite">
      <span class="spinner" aria-hidden="true"></span>
      <span>Sending request…</span>
    </div>
  {:else if !response}
    <div class="state empty">
      <div class="empty-icon" aria-hidden="true">⚡</div>
      <p>Send a request to see the response.</p>
      <p class="muted">Enter a URL and hit <b>Send</b>.</p>
    </div>
  {:else if hasError}
    <div class="error-wrap" role="alert">
      <div class="error-label">Request failed</div>
      <pre class="error-body">{response.error}</pre>
    </div>
  {:else}
    <div class="tabs" role="tablist" aria-label="Response sections">
      <button
        class="tab"
        class:active={activeTab === 'body'}
        role="tab"
        aria-selected={activeTab === 'body'}
        onclick={() => uiStore.setResponseTab('body')}
      >Body</button>
      <button
        class="tab"
        class:active={activeTab === 'headers'}
        role="tab"
        aria-selected={activeTab === 'headers'}
        onclick={() => uiStore.setResponseTab('headers')}
      >
        Headers <span class="badge">{response.headers.length}</span>
      </button>
    </div>

    {#if activeTab === 'body'}
      <BodyView {response} />
    {:else}
      <HeadersView headers={response.headers} />
    {/if}
  {/if}
</section>

<style>
  .response {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    border-top: 1px solid var(--border);
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    padding: var(--space-2) var(--space-4);
    border-bottom: 1px solid var(--border);
  }
  .label {
    color: var(--text-dim);
    font-size: var(--font-size-sm);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  /* StatusBar carries its own bottom border; remove the duplicate inside head. */
  .head :global(.statusbar) {
    border-bottom: none;
    padding-top: 0;
    padding-bottom: 0;
  }

  .state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    color: var(--text-dim);
    font-size: var(--font-size-md);
  }
  .empty-icon {
    font-size: var(--font-size-2xl);
    color: var(--accent);
    opacity: 0.6;
  }
  .empty p { margin: 0; font-size: var(--font-size-sm); }
  .muted { color: var(--text-dim); }

  .spinner {
    width: 16px;
    height: 16px;
    border: 2px solid var(--border-strong);
    border-top-color: var(--accent);
    display: inline-block;
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .error-wrap {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    padding: var(--space-4);
    gap: var(--space-2);
  }
  .error-label {
    color: var(--red);
    font-weight: var(--font-weight-semibold);
    font-size: var(--font-size-sm);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .error-body {
    flex: 1;
    margin: 0;
    overflow: auto;
    padding: var(--space-3) var(--space-4);
    background: var(--bg-elev-2);
    border: 1px solid var(--border);
    color: var(--red);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    line-height: var(--line-height-normal);
    white-space: pre-wrap;
    word-break: break-word;
    user-select: text;
  }

  .tabs {
    display: flex;
    gap: var(--space-1);
    padding: var(--space-1) var(--space-4) 0;
    border-bottom: 1px solid var(--border);
  }
  .tab {
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    color: var(--text-dim);
    padding: var(--space-2) var(--space-2);
    cursor: pointer;
    font-size: var(--font-size-sm);
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .tab:hover { color: var(--text); }
  .tab.active {
    color: var(--text);
    border-bottom-color: var(--accent);
  }
  .badge {
    background: var(--bg-elev-2);
    color: var(--accent);
    font-size: var(--font-size-xs);
    padding: 0 var(--space-2);
  }
</style>
