<script lang="ts">
  // BodyEditor — request body configuration bound to requestStore.current.body.
  // The body-type selector defaults to None (from emptyBodySpec()); raw JSON and
  // plain text use a textarea, while form-data and x-www-form-urlencoded use an
  // editable key/value table over body.formFields (Req 2.1).
  //
  // The Prettify control runs the shared, pure prettifyJSON helper on the raw
  // body: valid JSON is reformatted with two-space indentation (Req 2.10); invalid
  // JSON is left unchanged and an inline invalid-JSON indication is shown (Req 2.11).
  import { requestStore } from '../../lib/stores'
  import { emptyKeyValue, type BodyType, type KeyValue } from '../../lib/models'
  import { prettifyJSON } from '../../lib/json'
  import UnresolvedHint from './UnresolvedHint.svelte'

  const body = $derived(requestStore.current.body)

  // Inline indication shown when the user prettifies content that is not valid
  // JSON; cleared whenever the body changes or a valid prettify succeeds.
  let invalidJSON = $state(false)

  function setType(type: BodyType): void {
    invalidJSON = false
    requestStore.setBody({ ...requestStore.current.body, type })
  }

  function setRaw(raw: string): void {
    invalidJSON = false
    requestStore.setBody({ ...requestStore.current.body, raw })
  }

  function prettify(): void {
    const result = prettifyJSON(requestStore.current.body.raw)
    if (result.ok) {
      invalidJSON = false
      requestStore.setBody({ ...requestStore.current.body, raw: result.text })
    } else {
      // Leave the content unchanged; surface the invalid-JSON indication.
      invalidJSON = true
    }
  }

  // --- form-field table (form-data / urlencoded) ---------------------------
  const formRows = $derived<KeyValue[]>(
    (() => {
      const fields = body.formFields
      const last = fields[fields.length - 1]
      if (!last || last.key !== '' || last.value !== '') {
        return [...fields, emptyKeyValue()]
      }
      return fields
    })(),
  )

  function commitFields(next: KeyValue[]): void {
    requestStore.setBody({ ...requestStore.current.body, formFields: next })
  }

  function onFieldInput(index: number, field: 'key' | 'value', value: string): void {
    commitFields(formRows.map((r, i) => (i === index ? { ...r, [field]: value } : r)))
  }

  function onFieldToggle(index: number, enabled: boolean): void {
    commitFields(formRows.map((r, i) => (i === index ? { ...r, enabled } : r)))
  }

  function removeField(index: number): void {
    commitFields(formRows.filter((_, i) => i !== index))
  }

  function isTrailingField(index: number, row: KeyValue): boolean {
    return index === formRows.length - 1 && row.key === '' && row.value === ''
  }

  const isRaw = $derived(body.type === 'json' || body.type === 'text')
  const isForm = $derived(body.type === 'form-data' || body.type === 'urlencoded')
</script>

<div class="body-editor">
  <div class="body-toolbar">
    <label class="type-select">
      <span class="muted">Body</span>
      <select value={body.type} onchange={(e) => setType(e.currentTarget.value as BodyType)}>
        <option value="none">None</option>
        <option value="json">Raw JSON</option>
        <option value="text">Plain Text</option>
        <option value="form-data">form-data</option>
        <option value="urlencoded">x-www-form-urlencoded</option>
      </select>
    </label>

    {#if body.type === 'json'}
      <div class="toolbar-right">
        {#if invalidJSON}
          <span class="invalid" role="status">Invalid JSON — left unchanged</span>
        {/if}
        <button class="mini" onclick={prettify}>Prettify</button>
      </div>
    {/if}
  </div>

  {#if body.type === 'none'}
    <p class="muted hint">This request does not send a body.</p>
  {:else if isRaw}
    <textarea
      class="editor"
      value={body.raw}
      placeholder={body.type === 'json' ? '{\n  "key": "value"\n}' : 'Plain text body'}
      spellcheck="false"
      oninput={(e) => setRaw(e.currentTarget.value)}
    ></textarea>
    <UnresolvedHint texts={[body.raw]} label="Unresolved body" />
  {:else if isForm}
    <div class="kvtable">
      <div class="kvhead">
        <span></span>
        <span>Key</span>
        <span>Value</span>
        <span></span>
      </div>
      {#each formRows as row, i (i)}
        <div class="kvrow" class:disabled={!row.enabled}>
          <input
            type="checkbox"
            checked={row.enabled}
            aria-label="Enable field"
            onchange={(e) => onFieldToggle(i, e.currentTarget.checked)}
          />
          <input
            class="kv"
            value={row.key}
            placeholder="field"
            spellcheck="false"
            autocomplete="off"
            oninput={(e) => onFieldInput(i, 'key', e.currentTarget.value)}
          />
          <input
            class="kv"
            value={row.value}
            placeholder="value"
            spellcheck="false"
            autocomplete="off"
            oninput={(e) => onFieldInput(i, 'value', e.currentTarget.value)}
          />
          {#if !isTrailingField(i, row)}
            <button class="kvdel" title="Remove field" onclick={() => removeField(i)}>✕</button>
          {:else}
            <span></span>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .body-editor {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .body-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-3);
  }
  .type-select {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--font-size-md);
  }
  .type-select select {
    background: var(--bg-elev);
    border: 1px solid var(--border);
    padding: var(--space-1) var(--space-2);
    outline: none;
    font-size: var(--font-size-md);
  }
  .type-select select:focus {
    border-color: var(--accent);
  }
  .type-select select option {
    background: var(--bg-elev-2);
    color: var(--text);
  }
  .toolbar-right {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }
  .invalid {
    color: var(--red);
    font-size: var(--font-size-sm);
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
  .editor {
    width: 100%;
    min-height: 160px;
    resize: vertical;
    background: var(--bg-elev);
    border: 1px solid var(--border);
    padding: var(--space-3);
    font-family: var(--font-mono);
    font-size: var(--font-size-md);
    outline: none;
    line-height: var(--line-height-normal);
  }
  .editor:focus {
    border-color: var(--accent);
  }
  .muted {
    color: var(--text-dim);
  }
  .hint {
    margin: 0;
    font-size: var(--font-size-sm);
  }

  /* form-field key/value table */
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
