<script lang="ts">
  // MethodSelect — the HTTP method dropdown for the request editor.
  //
  // Lists every method Volt supports (Requirement 1.1) and binds directly to the
  // working request in requestStore. GET is the default because emptyRawRequest()
  // initialises `method` to 'GET'. The selected method is colour-coded using the
  // shared methodColor token map so it reads the same here as in tabs/history.
  import { requestStore } from '../../lib/stores'
  import { methodColor } from '../../lib/types'
  import type { Method } from '../../lib/models'

  // Typed list (the loosely-typed METHODS in types.ts is string[]); keeping a
  // Method[] here keeps the two-way binding type-clean.
  const METHODS: Method[] = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']

  const color = $derived(methodColor(requestStore.current.method))
</script>

<div class="method-select" style="border-color:{color}">
  <select
    aria-label="HTTP method"
    bind:value={requestStore.current.method}
    style="color:{color}"
  >
    {#each METHODS as m (m)}
      <option value={m}>{m}</option>
    {/each}
  </select>
</div>

<style>
  .method-select {
    border: 1px solid var(--border);
    border-left-width: 3px;
    background: var(--bg-elev);
    display: flex;
  }
  .method-select select {
    background: transparent;
    border: none;
    outline: none;
    padding: 0 var(--space-3);
    font-weight: var(--font-weight-bold);
    font-size: var(--font-size-md);
    font-family: var(--font-sans);
    cursor: pointer;
    min-width: 92px;
  }
  .method-select option {
    background: var(--bg-elev-2);
    color: var(--text);
  }
</style>
