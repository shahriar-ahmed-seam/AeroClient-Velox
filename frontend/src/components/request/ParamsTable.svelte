<script lang="ts">
  // ParamsTable — an editable table of query parameters (key / value / enabled)
  // bound to the working request's params in requestStore.
  //
  // Any edit (key, value, enabled toggle, add, remove) rewrites the raw URL's
  // query string from the enabled, non-empty params (Requirement 1.8) via the
  // pure applyParamsToUrl helper. Because the rewrite is a programmatic store
  // update — not a URL input event — it does not re-trigger UrlBar's parse, so
  // editing here and editing the URL there stay in sync without a loop.
  import { requestStore } from '../../lib/stores'
  import { applyParamsToUrl } from '../../lib/urlParams'
  import { emptyKeyValue, type KeyValue } from '../../lib/models'

  // Keep a trailing empty row so there is always a blank line to type into. The
  // empty row has no key, so it is excluded from the URL and from any send.
  $effect(() => {
    const params = requestStore.current.params
    const last = params[params.length - 1]
    if (!last || last.key !== '' || last.value !== '') {
      requestStore.setParams([...params, emptyKeyValue()])
    }
  })

  /** Push the new param list to the store and rewrite the URL query string. */
  function commit(params: KeyValue[]): void {
    requestStore.setParams(params)
    requestStore.setUrl(applyParamsToUrl(requestStore.current.url, params))
  }

  function updateRow(index: number, patch: Partial<KeyValue>): void {
    const params = requestStore.current.params.map((row, i) =>
      i === index ? { ...row, ...patch } : row,
    )
    commit(params)
  }

  function removeRow(index: number): void {
    commit(requestStore.current.params.filter((_, i) => i !== index))
  }
</script>

<div class="kvtable">
  <div class="kvhead">
    <span></span>
    <span>Key</span>
    <span>Value</span>
    <span></span>
  </div>

  {#each requestStore.current.params as row, i (i)}
    <div class="kvrow">
      <input
        type="checkbox"
        aria-label="Enable parameter"
        checked={row.enabled}
        onchange={(e) => updateRow(i, { enabled: (e.currentTarget as HTMLInputElement).checked })}
      />
      <input
        class="kv"
        placeholder="parameter"
        spellcheck="false"
        value={row.key}
        oninput={(e) => updateRow(i, { key: (e.currentTarget as HTMLInputElement).value })}
      />
      <input
        class="kv"
        placeholder="value"
        spellcheck="false"
        value={row.value}
        oninput={(e) => updateRow(i, { value: (e.currentTarget as HTMLInputElement).value })}
      />
      <button class="kvdel" aria-label="Remove parameter" onclick={() => removeRow(i)}>✕</button>
    </div>
  {/each}
</div>

<style>
  .kvtable {
    display: flex;
    flex-direction: column;
    font-size: var(--font-size-md);
  }
  .kvhead,
  .kvrow {
    display: grid;
    grid-template-columns: 28px 1fr 1fr 30px;
    align-items: center;
  }
  .kvhead {
    color: var(--text-dim);
    font-size: var(--font-size-xs);
    padding: var(--space-1) 0;
    border-bottom: 1px solid var(--border);
  }
  .kvrow {
    border-bottom: 1px solid var(--border);
  }
  .kv {
    background: transparent;
    border: none;
    outline: none;
    color: var(--text);
    padding: var(--space-2);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
  }
  .kv::placeholder {
    color: var(--text-dim);
  }
  .kvdel {
    background: transparent;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    font-size: var(--font-size-xs);
  }
  .kvdel:hover {
    color: var(--red);
  }
</style>
