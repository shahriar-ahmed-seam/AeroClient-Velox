<script lang="ts">
  // HeadersTable — editable key/value/enabled rows bound to the working request's
  // headers (requestStore.current.headers). Supports adding (via a persistent
  // trailing empty row) and removing rows. Disabled or empty-key rows are kept in
  // the editor but the engine ignores them at send time (Req 1.5/1.6); this
  // component is purely the editing surface for Req 2.1-style key/value tables.
  import { requestStore } from '../../lib/stores'
  import { emptyKeyValue, type KeyValue } from '../../lib/models'

  // Bindable view of the headers list. Mutating array items is reactive because
  // requestStore.current is a $state proxy.
  const headers = $derived(requestStore.current.headers)

  // Always render one trailing blank row so the user can type into a new line
  // without an explicit "add" click. The blank row is only persisted once the
  // user types into it (and a fresh blank row appears after it).
  const rows = $derived<KeyValue[]>(
    (() => {
      const last = headers[headers.length - 1]
      if (!last || last.key !== '' || last.value !== '') {
        return [...headers, emptyKeyValue()]
      }
      return headers
    })(),
  )

  function commit(next: KeyValue[]): void {
    // Drop trailing fully-empty rows except keep the list itself; the derived
    // `rows` re-adds a single editing row.
    requestStore.setHeaders(next)
  }

  function onInput(index: number, field: 'key' | 'value', value: string): void {
    const next = rows.map((r, i) => (i === index ? { ...r, [field]: value } : r))
    commit(next)
  }

  function onToggle(index: number, enabled: boolean): void {
    const next = rows.map((r, i) => (i === index ? { ...r, enabled } : r))
    commit(next)
  }

  function removeRow(index: number): void {
    commit(rows.filter((_, i) => i !== index))
  }

  // A row is the "new" trailing editing row when it is the last one and empty.
  function isTrailing(index: number, row: KeyValue): boolean {
    return index === rows.length - 1 && row.key === '' && row.value === ''
  }
</script>

<div class="kvtable">
  <div class="kvhead">
    <span></span>
    <span>Header</span>
    <span>Value</span>
    <span></span>
  </div>
  {#each rows as row, i (i)}
    <div class="kvrow" class:disabled={!row.enabled}>
      <input
        type="checkbox"
        checked={row.enabled}
        aria-label="Enable header"
        onchange={(e) => onToggle(i, e.currentTarget.checked)}
      />
      <input
        class="kv"
        value={row.key}
        placeholder="header"
        spellcheck="false"
        autocomplete="off"
        oninput={(e) => onInput(i, 'key', e.currentTarget.value)}
      />
      <input
        class="kv"
        value={row.value}
        placeholder="value"
        spellcheck="false"
        autocomplete="off"
        oninput={(e) => onInput(i, 'value', e.currentTarget.value)}
      />
      {#if !isTrailing(i, row)}
        <button class="kvdel" title="Remove header" onclick={() => removeRow(i)}>✕</button>
      {:else}
        <span></span>
      {/if}
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
  .kvrow.disabled .kv {
    color: var(--text-dim);
    text-decoration: line-through;
  }
  .kv {
    background: transparent;
    border: none;
    outline: none;
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
