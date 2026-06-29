<script lang="ts">
  // CollectionsTree renders the persistent collection tree (collections >
  // folders > requests) and is the sidebar surface for organizing saved
  // requests. Every mutation is routed through collectionsStore, which forwards
  // to the Backend and reloads the authoritative tree; this component never
  // mutates persisted data directly.
  //
  // Behavior mapped to requirements:
  //  - Create / rename / delete collections and folders (Req 5.2, 5.3).
  //  - Move a saved request into another collection or folder (Req 5.4).
  //  - Deleting a container that holds folders or requests prompts for
  //    confirmation before deleting; declining leaves it unchanged (Req 5.7).
  //  - Names are validated 1..255 chars inline before any store call (Req 5.8).
  //  - Selecting a saved request restores its full configuration into the
  //    editor via requestStore.loadConfig (Req 7.4).
  //  - A failing store operation surfaces through uiStore.errorMessage and the
  //    user's in-progress input is preserved, never cleared (Req 11.7).

  import { collectionsStore } from '../../lib/stores/collectionsStore.svelte'
  import { requestStore } from '../../lib/stores/requestStore.svelte'
  import { uiStore } from '../../lib/stores/uiStore.svelte'
  import type { Collection, Folder, SavedRequest } from '../../lib/models'

  const MAX_NAME = 255
  const MAX_DEPTH = 10 // folders nest up to 10 levels deep (Req 5.3)

  // -- transient editor state ------------------------------------------------

  let newCollectionName = $state('')
  let creatingCollection = $state(false)

  // A single rename slot — only one item is renamed at a time.
  let renamingId = $state<string | null>(null)
  let renameDraft = $state('')

  // Adding a folder under a given container id.
  let addingFolderFor = $state<string | null>(null)
  let newFolderName = $state('')

  // Moving a request: the request id whose move-target picker is open.
  let movingRequestId = $state<string | null>(null)

  // Inline validation message for the currently active form, or null.
  let validationError = $state<string | null>(null)

  // Track which containers are collapsed (by id). Default expanded.
  let collapsed = $state<Record<string, boolean>>({})

  const tree = $derived(collectionsStore.tree)

  // Flat list of every container (collection or folder) used as move targets.
  interface MoveTarget {
    id: string
    label: string
  }

  const moveTargets = $derived<MoveTarget[]>(buildMoveTargets(tree))

  function buildMoveTargets(collections: Collection[]): MoveTarget[] {
    const out: MoveTarget[] = []
    for (const c of collections) {
      out.push({ id: c.id, label: c.name })
      walkFolders(c.folders, c.name, out)
    }
    return out
  }

  function walkFolders(folders: Folder[], prefix: string, out: MoveTarget[]): void {
    for (const f of folders) {
      const label = `${prefix} / ${f.name}`
      out.push({ id: f.id, label })
      walkFolders(f.folders, label, out)
    }
  }

  // -- helpers ---------------------------------------------------------------

  /** Validate a candidate name (1..255). Returns an error message or null. */
  function nameError(name: string): string | null {
    const trimmed = name.trim()
    if (trimmed.length === 0) return 'Name is required'
    if (name.length > MAX_NAME) return `Name must be ${MAX_NAME} characters or fewer`
    return null
  }

  function isNonEmptyContainer(node: Collection | Folder): boolean {
    return node.folders.length > 0 || node.requests.length > 0
  }

  function toggleCollapse(id: string): void {
    collapsed[id] = !collapsed[id]
  }

  /**
   * Run a store mutation and report whether it succeeded. collectionsStore
   * swallows Backend failures into uiStore.errorMessage, so we clear the error
   * first and treat a still-empty error as success. On failure we leave all
   * form state intact so the user's input is preserved (Req 11.7).
   */
  async function runMutation(op: () => Promise<void>): Promise<boolean> {
    uiStore.clearError()
    await op()
    return uiStore.errorMessage === null
  }

  // -- create collection -----------------------------------------------------

  function startCreateCollection(): void {
    creatingCollection = true
    newCollectionName = ''
    validationError = null
  }

  function cancelCreateCollection(): void {
    creatingCollection = false
    newCollectionName = ''
    validationError = null
  }

  async function submitCreateCollection(): Promise<void> {
    const err = nameError(newCollectionName)
    if (err) {
      validationError = err
      return
    }
    validationError = null
    const collection: Collection = {
      id: '',
      name: newCollectionName.trim(),
      folders: [],
      requests: [],
      order: tree.length,
    }
    const ok = await runMutation(() => collectionsStore.saveCollection(collection))
    if (ok) cancelCreateCollection()
  }

  // -- rename ----------------------------------------------------------------

  function startRename(id: string, currentName: string): void {
    renamingId = id
    renameDraft = currentName
    validationError = null
  }

  function cancelRename(): void {
    renamingId = null
    renameDraft = ''
    validationError = null
  }

  async function submitRenameCollection(id: string): Promise<void> {
    const err = nameError(renameDraft)
    if (err) {
      validationError = err
      return
    }
    validationError = null
    const ok = await runMutation(() => collectionsStore.renameCollection(id, renameDraft.trim()))
    if (ok) cancelRename()
  }

  async function submitRenameFolder(folder: Folder): Promise<void> {
    const err = nameError(renameDraft)
    if (err) {
      validationError = err
      return
    }
    validationError = null
    const updated: Folder = { ...folder, name: renameDraft.trim() }
    // Folders are saved through the store; the parent id is unchanged so we
    // resolve it from the existing tree position.
    const parentId = findFolderParentId(tree, folder.id)
    if (parentId == null) {
      validationError = 'Could not locate folder'
      return
    }
    const ok = await runMutation(() => collectionsStore.saveFolder(updated, parentId))
    if (ok) cancelRename()
  }

  /** Find the id of the container directly holding the folder with folderId. */
  function findFolderParentId(collections: Collection[], folderId: string): string | null {
    for (const c of collections) {
      const r = searchFolderParent(c.folders, c.id, folderId)
      if (r != null) return r
    }
    return null
  }

  function searchFolderParent(folders: Folder[], parentId: string, folderId: string): string | null {
    for (const f of folders) {
      if (f.id === folderId) return parentId
      const deeper = searchFolderParent(f.folders, f.id, folderId)
      if (deeper != null) return deeper
    }
    return null
  }

  // -- add folder ------------------------------------------------------------

  function startAddFolder(containerId: string): void {
    addingFolderFor = containerId
    newFolderName = ''
    validationError = null
  }

  function cancelAddFolder(): void {
    addingFolderFor = null
    newFolderName = ''
    validationError = null
  }

  async function submitAddFolder(containerId: string): Promise<void> {
    const err = nameError(newFolderName)
    if (err) {
      validationError = err
      return
    }
    validationError = null
    const folder: Folder = { id: '', name: newFolderName.trim(), folders: [], requests: [] }
    const ok = await runMutation(() => collectionsStore.saveFolder(folder, containerId))
    if (ok) cancelAddFolder()
  }

  // -- delete ----------------------------------------------------------------

  async function deleteCollection(c: Collection): Promise<void> {
    if (isNonEmptyContainer(c)) {
      const confirmed = window.confirm(
        `Delete collection "${c.name}" and everything inside it? This cannot be undone.`,
      )
      if (!confirmed) return // declined: leave unchanged (Req 5.7)
    }
    await runMutation(() => collectionsStore.deleteCollection(c.id))
  }

  async function deleteFolder(f: Folder): Promise<void> {
    if (isNonEmptyContainer(f)) {
      const confirmed = window.confirm(
        `Delete folder "${f.name}" and everything inside it? This cannot be undone.`,
      )
      if (!confirmed) return // declined: leave unchanged (Req 5.7)
    }
    await runMutation(() => collectionsStore.deleteFolder(f.id))
  }

  async function deleteRequest(req: SavedRequest): Promise<void> {
    await runMutation(() => collectionsStore.deleteRequest(req.id))
  }

  // -- move request ----------------------------------------------------------

  function startMove(requestId: string): void {
    movingRequestId = movingRequestId === requestId ? null : requestId
  }

  async function moveRequestTo(requestId: string, targetParentId: string): Promise<void> {
    if (!targetParentId) return
    const ok = await runMutation(() => collectionsStore.moveRequest(requestId, targetParentId))
    if (ok) movingRequestId = null
  }

  // -- select (restore config) ----------------------------------------------

  function selectRequest(req: SavedRequest): void {
    requestStore.loadConfig(req) // Req 7.4
  }

  function depthAllowsFolder(depth: number): boolean {
    return depth < MAX_DEPTH
  }
</script>

<div class="collections" role="tree" aria-label="Collections">
  <div class="head">
    <span class="title">Collections</span>
    <button class="mini" onclick={startCreateCollection} title="New collection">+ New</button>
  </div>

  {#if creatingCollection}
    <div class="inline-form">
      <input
        class="field"
        bind:value={newCollectionName}
        placeholder="Collection name"
        spellcheck="false"
        onkeydown={(e) => e.key === 'Enter' && submitCreateCollection()}
      />
      <button class="mini" onclick={submitCreateCollection}>Save</button>
      <button class="mini ghost" onclick={cancelCreateCollection}>Cancel</button>
    </div>
    {#if validationError}<div class="err">{validationError}</div>{/if}
  {/if}

  {#if tree.length === 0 && !creatingCollection}
    <div class="empty">
      <p>No collections yet.</p>
      <p class="dim">Create one to start saving requests.</p>
    </div>
  {/if}

  <div class="tree">
    {#each tree as collection (collection.id)}
      <div class="node">
        <div class="row">
          <button
            class="twisty"
            onclick={() => toggleCollapse(collection.id)}
            aria-label={collapsed[collection.id] ? 'Expand' : 'Collapse'}
          >
            {collapsed[collection.id] ? '▸' : '▾'}
          </button>

          {#if renamingId === collection.id}
            <input
              class="field"
              bind:value={renameDraft}
              spellcheck="false"
              onkeydown={(e) => e.key === 'Enter' && submitRenameCollection(collection.id)}
            />
            <button class="mini" onclick={() => submitRenameCollection(collection.id)}>Save</button>
            <button class="mini ghost" onclick={cancelRename}>Cancel</button>
          {:else}
            <span class="label collection-label">{collection.name}</span>
            <div class="actions">
              <button class="act" title="Add folder" onclick={() => startAddFolder(collection.id)}>📁+</button>
              <button class="act" title="Rename" onclick={() => startRename(collection.id, collection.name)}>✎</button>
              <button class="act danger" title="Delete" onclick={() => deleteCollection(collection)}>✕</button>
            </div>
          {/if}
        </div>

        {#if renamingId === collection.id && validationError}
          <div class="err indented">{validationError}</div>
        {/if}

        {#if !collapsed[collection.id]}
          {#if addingFolderFor === collection.id}
            <div class="inline-form indented">
              <input
                class="field"
                bind:value={newFolderName}
                placeholder="Folder name"
                spellcheck="false"
                onkeydown={(e) => e.key === 'Enter' && submitAddFolder(collection.id)}
              />
              <button class="mini" onclick={() => submitAddFolder(collection.id)}>Save</button>
              <button class="mini ghost" onclick={cancelAddFolder}>Cancel</button>
            </div>
            {#if validationError}<div class="err indented">{validationError}</div>{/if}
          {/if}

          {@render containerBody(collection.folders, collection.requests, 1)}
        {/if}
      </div>
    {/each}
  </div>
</div>

<!--
  Recursive snippet rendering a container's folders and requests. `depth`
  starts at 1 for a collection's direct children and increments per folder
  level so the add-folder control can respect the 10-level nesting bound.
-->
{#snippet containerBody(folders: Folder[], requests: SavedRequest[], depth: number)}
  <div class="children" style="--depth: {depth}">
    {#each folders as folder (folder.id)}
      <div class="node">
        <div class="row">
          <button
            class="twisty"
            onclick={() => toggleCollapse(folder.id)}
            aria-label={collapsed[folder.id] ? 'Expand' : 'Collapse'}
          >
            {collapsed[folder.id] ? '▸' : '▾'}
          </button>

          {#if renamingId === folder.id}
            <input
              class="field"
              bind:value={renameDraft}
              spellcheck="false"
              onkeydown={(e) => e.key === 'Enter' && submitRenameFolder(folder)}
            />
            <button class="mini" onclick={() => submitRenameFolder(folder)}>Save</button>
            <button class="mini ghost" onclick={cancelRename}>Cancel</button>
          {:else}
            <span class="label folder-label">{folder.name}</span>
            <div class="actions">
              {#if depthAllowsFolder(depth)}
                <button class="act" title="Add subfolder" onclick={() => startAddFolder(folder.id)}>📁+</button>
              {/if}
              <button class="act" title="Rename" onclick={() => startRename(folder.id, folder.name)}>✎</button>
              <button class="act danger" title="Delete" onclick={() => deleteFolder(folder)}>✕</button>
            </div>
          {/if}
        </div>

        {#if renamingId === folder.id && validationError}
          <div class="err indented">{validationError}</div>
        {/if}

        {#if !collapsed[folder.id]}
          {#if addingFolderFor === folder.id}
            <div class="inline-form indented">
              <input
                class="field"
                bind:value={newFolderName}
                placeholder="Folder name"
                spellcheck="false"
                onkeydown={(e) => e.key === 'Enter' && submitAddFolder(folder.id)}
              />
              <button class="mini" onclick={() => submitAddFolder(folder.id)}>Save</button>
              <button class="mini ghost" onclick={cancelAddFolder}>Cancel</button>
            </div>
            {#if validationError}<div class="err indented">{validationError}</div>{/if}
          {/if}

          {@render containerBody(folder.folders, folder.requests, depth + 1)}
        {/if}
      </div>
    {/each}

    {#each requests as req (req.id)}
      <div class="req-node">
        <div class="row">
          <button class="req" onclick={() => selectRequest(req)} title="Open request">
            <span class="method method-{req.method.toLowerCase()}">{req.method}</span>
            <span class="req-name">{req.name}</span>
          </button>
          <div class="actions">
            <button class="act" title="Move" onclick={() => startMove(req.id)}>⇄</button>
            <button class="act danger" title="Delete" onclick={() => deleteRequest(req)}>✕</button>
          </div>
        </div>

        {#if movingRequestId === req.id}
          <div class="inline-form indented">
            <select
              class="field"
              onchange={(e) => moveRequestTo(req.id, (e.currentTarget as HTMLSelectElement).value)}
            >
              <option value="">Move to…</option>
              {#each moveTargets as t (t.id)}
                <option value={t.id}>{t.label}</option>
              {/each}
            </select>
            <button class="mini ghost" onclick={() => (movingRequestId = null)}>Cancel</button>
          </div>
        {/if}
      </div>
    {/each}
  </div>
{/snippet}

<style>
  .collections {
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

  .tree {
    display: flex;
    flex-direction: column;
  }

  .children {
    margin-left: var(--space-3);
    border-left: 1px solid var(--border);
    padding-left: var(--space-2);
  }

  .row {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    min-height: 28px;
  }

  .twisty {
    background: transparent;
    border: none;
    color: var(--text-dim);
    cursor: pointer;
    font-size: var(--font-size-xs);
    padding: 0 var(--space-1);
    width: 18px;
  }

  .label {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--font-size-md);
  }

  .collection-label {
    font-weight: var(--font-weight-semibold);
  }

  .folder-label {
    color: var(--text);
  }

  .actions {
    display: flex;
    gap: var(--space-1);
    opacity: 0;
  }

  .row:hover .actions,
  .req-node:hover .actions {
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

  .req {
    flex: 1;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    background: transparent;
    border: none;
    cursor: pointer;
    text-align: left;
    padding: var(--space-1) 0;
    overflow: hidden;
  }

  .req:hover .req-name {
    color: var(--text);
  }

  .req-name {
    color: var(--text-dim);
    font-size: var(--font-size-sm);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .method {
    font-weight: var(--font-weight-bold);
    font-size: var(--font-size-xs);
    min-width: 42px;
  }

  .method-get { color: var(--green); }
  .method-post { color: var(--accent); }
  .method-put { color: var(--blue); }
  .method-patch { color: var(--purple); }
  .method-delete { color: var(--red); }
  .method-head,
  .method-options { color: var(--text-dim); }

  .inline-form {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-1) 0;
  }

  .inline-form.indented {
    margin-left: var(--space-4);
  }

  .field {
    flex: 1;
    min-width: 0;
    background: var(--bg-elev);
    border: 1px solid var(--border);
    color: var(--text);
    padding: var(--space-1) var(--space-2);
    font-size: var(--font-size-sm);
    outline: none;
  }

  .field:focus {
    border-color: var(--accent);
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

  .mini.ghost {
    background: transparent;
  }

  .err {
    color: var(--red);
    font-size: var(--font-size-xs);
    padding: var(--space-1) 0;
  }

  .err.indented {
    margin-left: var(--space-4);
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
    color: var(--text-dim);
    opacity: 0.7;
    font-size: var(--font-size-xs);
  }

  .dim {
    color: var(--text-dim);
  }
</style>
