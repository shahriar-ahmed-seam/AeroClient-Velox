<script lang="ts">
  // EnvironmentManager lists the persisted environments and is the sidebar
  // surface for creating, editing, activating, and deleting them along with
  // their variables. Every mutation routes through environmentsStore, which
  // forwards to the Backend and reloads the list so the single-active invariant
  // and variable edits always reflect persisted state.
  //
  // Behavior mapped to requirements:
  //  - Create / rename / delete environments, names 1..64 and unique (Req 6.1).
  //  - Define / edit / remove variables, names 1..128 unique within the
  //    environment, values 0..4096 (Req 6.2).
  //  - Activate exactly one environment at a time; deactivating clears the
  //    active selection, and the store clears active when the active
  //    environment is deleted (Req 6.3, 6.10).
  //  - Invalid names are rejected inline before any store call (Req 6.9), and a
  //    failing store operation surfaces through uiStore.errorMessage while the
  //    in-progress draft is preserved, never cleared (Req 11.7).

  import { environmentsStore } from '../../lib/stores/environmentsStore.svelte'
  import { uiStore } from '../../lib/stores/uiStore.svelte'
  import type { Environment, Variable } from '../../lib/models'

  const MAX_ENV_NAME = 64
  const MAX_VAR_NAME = 128
  const MAX_VAR_VALUE = 4096

  const environments = $derived(environmentsStore.environments)

  // The environment currently open in the editor (a working copy), or null.
  // id === '' indicates a brand-new environment being created.
  let draft = $state<Environment | null>(null)

  // -- validation ------------------------------------------------------------

  const envNameValidation = $derived(draft ? envNameError(draft.name, draft.id) : null)
  const variableValidation = $derived(draft ? variableErrors(draft.variables) : [])
  const draftHasErrors = $derived(
    envNameValidation != null || variableValidation.some((e) => e != null),
  )

  function envNameError(name: string, draftId: string): string | null {
    const trimmed = name.trim()
    if (trimmed.length === 0) return 'Name is required'
    if (name.length > MAX_ENV_NAME) return `Name must be ${MAX_ENV_NAME} characters or fewer`
    const duplicate = environments.some((e) => e.id !== draftId && e.name === trimmed)
    if (duplicate) return 'An environment with this name already exists'
    return null
  }

  /** A row is blank (and ignored / dropped on save) when both fields are empty. */
  function isBlankVar(v: Variable): boolean {
    return v.name.trim() === '' && v.value === ''
  }

  function variableErrors(vars: Variable[]): (string | null)[] {
    return vars.map((v, i) => {
      if (isBlankVar(v)) return null
      const trimmed = v.name.trim()
      if (trimmed.length === 0) return 'Name is required'
      if (v.name.length > MAX_VAR_NAME) return `Name must be ${MAX_VAR_NAME} characters or fewer`
      if (v.value.length > MAX_VAR_VALUE) return `Value must be ${MAX_VAR_VALUE} characters or fewer`
      const dup = vars.some((o, j) => j !== i && o.name.trim() !== '' && o.name.trim() === trimmed)
      if (dup) return 'Duplicate variable name'
      return null
    })
  }

  // -- helpers ---------------------------------------------------------------

  function cloneEnv(e: Environment): Environment {
    return { ...e, variables: e.variables.map((v) => ({ ...v })) }
  }

  /**
   * Run a store mutation and report whether it succeeded. environmentsStore
   * swallows Backend failures into uiStore.errorMessage, so we clear it first
   * and treat a still-empty error as success. On failure the draft is left
   * intact so the user's input is preserved (Req 11.7).
   */
  async function runMutation(op: () => Promise<void>): Promise<boolean> {
    uiStore.clearError()
    await op()
    return uiStore.errorMessage === null
  }

  // -- editor lifecycle ------------------------------------------------------

  function startCreate(): void {
    draft = { id: '', name: '', variables: [{ name: '', value: '' }], active: false }
  }

  function startEdit(env: Environment): void {
    const copy = cloneEnv(env)
    // Always offer a trailing blank row to type into.
    copy.variables.push({ name: '', value: '' })
    draft = copy
  }

  function cancelEdit(): void {
    draft = null
  }

  function addVariableRow(): void {
    if (draft) draft.variables.push({ name: '', value: '' })
  }

  function removeVariableRow(index: number): void {
    if (draft) draft.variables.splice(index, 1)
  }

  async function saveDraft(): Promise<void> {
    if (!draft || draftHasErrors) return
    const toSave: Environment = {
      ...draft,
      name: draft.name.trim(),
      variables: draft.variables
        .filter((v) => !isBlankVar(v))
        .map((v) => ({ name: v.name.trim(), value: v.value })),
    }
    const ok = await runMutation(() => environmentsStore.save(toSave))
    if (ok) draft = null // success: close. On failure keep draft (Req 11.7).
  }

  // -- activation / deletion -------------------------------------------------

  async function toggleActive(env: Environment): Promise<void> {
    // Activate this one, or clear active if it is already active (Req 6.3).
    await runMutation(() => environmentsStore.setActive(env.active ? '' : env.id))
  }

  async function deleteEnvironment(env: Environment): Promise<void> {
    const confirmed = window.confirm(`Delete environment "${env.name}"? This cannot be undone.`)
    if (!confirmed) return
    const ok = await runMutation(() => environmentsStore.delete(env.id))
    if (ok && draft?.id === env.id) draft = null
  }
</script>

<div class="environments">
  <div class="head">
    <span class="title">Environments</span>
    <button class="mini" onclick={startCreate} title="New environment">+ New</button>
  </div>

  {#if environments.length === 0 && draft == null}
    <div class="empty">
      <p>No environments yet.</p>
      <p class="dim">Create one to define {'{{variables}}'}.</p>
    </div>
  {/if}

  <div class="list">
    {#each environments as env (env.id)}
      <div class="env" class:active={env.active}>
        <div class="env-row">
          <button
            class="dot"
            class:on={env.active}
            onclick={() => toggleActive(env)}
            title={env.active ? 'Deactivate' : 'Activate'}
            aria-pressed={env.active}
          ></button>
          <span class="env-name">{env.name}</span>
          {#if env.active}<span class="badge">Active</span>{/if}
          <span class="count">{env.variables.length} vars</span>
          <div class="actions">
            <button class="act" title="Edit" onclick={() => startEdit(env)}>✎</button>
            <button class="act danger" title="Delete" onclick={() => deleteEnvironment(env)}>✕</button>
          </div>
        </div>
      </div>
    {/each}
  </div>

  {#if draft}
    <div class="editor">
      <div class="editor-head">{draft.id === '' ? 'New environment' : 'Edit environment'}</div>

      <label class="lbl" for="env-name">Name</label>
      <input
        id="env-name"
        class="field"
        bind:value={draft.name}
        placeholder="e.g. Production"
        spellcheck="false"
      />
      {#if envNameValidation}<div class="err">{envNameValidation}</div>{/if}

      <div class="vars-head">
        <span class="lbl">Variables</span>
        <button class="mini ghost" onclick={addVariableRow}>+ Add</button>
      </div>

      <div class="vars">
        {#each draft.variables as variable, i (i)}
          <div class="var-row">
            <input
              class="field"
              bind:value={variable.name}
              placeholder="name"
              spellcheck="false"
            />
            <input
              class="field"
              bind:value={variable.value}
              placeholder="value"
              spellcheck="false"
            />
            <button class="act danger" title="Remove" onclick={() => removeVariableRow(i)}>✕</button>
          </div>
          {#if variableValidation[i]}<div class="err">{variableValidation[i]}</div>{/if}
        {/each}
      </div>

      <div class="editor-actions">
        <button class="mini" onclick={saveDraft} disabled={draftHasErrors}>Save</button>
        <button class="mini ghost" onclick={cancelEdit}>Cancel</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .environments {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: var(--space-4);
    gap: var(--space-2);
    overflow: auto;
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

  .list {
    display: flex;
    flex-direction: column;
  }

  .env {
    border-bottom: 1px solid var(--border);
  }

  .env-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: 32px;
  }

  .dot {
    width: 12px;
    height: 12px;
    border: 1px solid var(--border-strong);
    background: transparent;
    cursor: pointer;
    flex: 0 0 auto;
    padding: 0;
  }

  .dot.on {
    background: var(--green);
    border-color: var(--green);
  }

  .env-name {
    flex: 1;
    font-size: var(--font-size-md);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .env.active .env-name {
    font-weight: var(--font-weight-semibold);
  }

  .badge {
    font-size: var(--font-size-xs);
    color: var(--green);
    border: 1px solid var(--green);
    padding: 0 var(--space-1);
  }

  .count {
    font-size: var(--font-size-xs);
    color: var(--text-dim);
  }

  .actions {
    display: flex;
    gap: var(--space-1);
    opacity: 0;
  }

  .env-row:hover .actions {
    opacity: 1;
  }

  .act {
    background: transparent;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    font-size: var(--font-size-xs);
    padding: var(--space-1);
  }

  .act:hover {
    color: var(--text);
  }

  .act.danger:hover {
    color: var(--red);
  }

  .editor {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin-top: var(--space-2);
    padding: var(--space-4);
    background: var(--bg-elev);
    border: 1px solid var(--border);
  }

  .editor-head {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    color: var(--text);
  }

  .lbl {
    font-size: var(--font-size-xs);
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .vars-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: var(--space-2);
  }

  .vars {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .var-row {
    display: grid;
    grid-template-columns: 1fr 1fr auto;
    align-items: center;
    gap: var(--space-2);
  }

  .field {
    min-width: 0;
    background: var(--bg);
    border: 1px solid var(--border);
    color: var(--text);
    padding: var(--space-2);
    font-size: var(--font-size-sm);
    outline: none;
  }

  .field:focus {
    border-color: var(--accent);
  }

  .editor-actions {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-2);
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

  .mini:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .mini.ghost {
    background: transparent;
  }

  .err {
    color: var(--red);
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
