<script lang="ts">
  // UrlBar — method selector + URL input + Send button for the request editor.
  //
  // Editing the URL re-derives the parameter table from the URL's query string
  // (Requirement 1.7). Because this only fires on the input event (a user edit),
  // it never feeds back into ParamsTable's URL rewrite, so the two stay in sync
  // without an update loop: URL edits drive params here, param edits drive the
  // URL in ParamsTable, and neither programmatic update re-triggers the other's
  // input handler.
  import { requestStore } from '../../lib/stores'
  import { parseQueryToParams } from '../../lib/urlParams'
  import MethodSelect from './MethodSelect.svelte'
  import UnresolvedHint from './UnresolvedHint.svelte'

  function onUrlInput(event: Event): void {
    const value = (event.currentTarget as HTMLInputElement).value
    requestStore.setUrl(value)
    // Reflect the query parameters now present in the URL into the table.
    requestStore.setParams(parseQueryToParams(value))
  }

  function send(): void {
    void requestStore.send()
  }
</script>

<div class="urlbar-wrap">
  <div class="urlbar">
    <MethodSelect />

    <input
      class="url-input"
      type="text"
      aria-label="Request URL"
      placeholder="https://api.example.com/endpoint"
      spellcheck="false"
      value={requestStore.current.url}
      oninput={onUrlInput}
    />

    <button class="send" onclick={send} disabled={requestStore.loading}>
      {requestStore.loading ? 'Sending…' : 'Send'}
    </button>
  </div>

  <UnresolvedHint texts={[requestStore.current.url]} label="Unresolved URL" />
</div>

<style>
  .urlbar-wrap {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
  }
  .urlbar {
    display: flex;
    gap: var(--space-2);
    align-items: stretch;
  }
  .url-input {
    flex: 1;
    min-width: 0;
    background: var(--bg-elev);
    border: 1px solid var(--border);
    padding: 0 var(--space-3);
    outline: none;
    color: var(--text);
    font-family: var(--font-mono);
    font-size: var(--font-size-md);
  }
  .url-input:focus {
    border-color: var(--accent);
  }
  .url-input::placeholder {
    color: var(--text-dim);
  }
  .send {
    background: var(--accent);
    color: var(--on-accent);
    border: none;
    font-weight: var(--font-weight-bold);
    font-family: var(--font-sans);
    padding: 0 var(--space-6);
    cursor: pointer;
    font-size: var(--font-size-md);
  }
  .send:hover {
    background: var(--accent-2);
  }
  .send:disabled {
    opacity: 0.6;
    cursor: default;
  }
</style>
