<script lang="ts">
  // AuthPanel — authorization configuration for the working request, bound to
  // requestStore.current.auth. Exactly one auth type is active at a time
  // (Req 3.1); the default is None as supplied by emptyAuthSpec(). Only the
  // fields relevant to the selected type are shown. This component edits the
  // AuthSpec only; the shared Go core derives the actual header/query at send
  // time (Req 3.2-3.9).
  import { requestStore } from '../../lib/stores'
  import type { ApiKeyLocation, AuthType } from '../../lib/models'

  const auth = $derived(requestStore.current.auth)

  function update<K extends keyof typeof auth>(field: K, value: (typeof auth)[K]): void {
    requestStore.setAuth({ ...requestStore.current.auth, [field]: value })
  }

  function onType(value: AuthType): void {
    update('type', value)
  }

  function onLocation(value: ApiKeyLocation): void {
    update('apiKeyLocation', value)
  }
</script>

<div class="auth">
  <label class="auth-row">
    <span>Type</span>
    <select value={auth.type} onchange={(e) => onType(e.currentTarget.value as AuthType)}>
      <option value="none">None</option>
      <option value="bearer">Bearer Token</option>
      <option value="basic">Basic Auth</option>
      <option value="apikey">API Key</option>
    </select>
  </label>

  {#if auth.type === 'none'}
    <p class="auth-hint">This request does not use authorization.</p>
  {:else if auth.type === 'bearer'}
    <label class="auth-row">
      <span>Token</span>
      <input
        class="field"
        value={auth.bearerToken}
        placeholder="token"
        spellcheck="false"
        autocomplete="off"
        oninput={(e) => update('bearerToken', e.currentTarget.value)}
      />
    </label>
  {:else if auth.type === 'basic'}
    <label class="auth-row">
      <span>Username</span>
      <input
        class="field"
        value={auth.basicUser}
        spellcheck="false"
        autocomplete="off"
        oninput={(e) => update('basicUser', e.currentTarget.value)}
      />
    </label>
    <label class="auth-row">
      <span>Password</span>
      <input
        class="field"
        type="password"
        value={auth.basicPass}
        autocomplete="off"
        oninput={(e) => update('basicPass', e.currentTarget.value)}
      />
    </label>
  {:else if auth.type === 'apikey'}
    <label class="auth-row">
      <span>Key</span>
      <input
        class="field"
        value={auth.apiKeyName}
        placeholder="X-API-Key"
        spellcheck="false"
        autocomplete="off"
        oninput={(e) => update('apiKeyName', e.currentTarget.value)}
      />
    </label>
    <label class="auth-row">
      <span>Value</span>
      <input
        class="field"
        value={auth.apiKeyValue}
        placeholder="value"
        spellcheck="false"
        autocomplete="off"
        oninput={(e) => update('apiKeyValue', e.currentTarget.value)}
      />
    </label>
    <label class="auth-row">
      <span>Add to</span>
      <select
        value={auth.apiKeyLocation}
        onchange={(e) => onLocation(e.currentTarget.value as ApiKeyLocation)}
      >
        <option value="header">Header</option>
        <option value="query">Query Params</option>
      </select>
    </label>
  {/if}
</div>

<style>
  .auth {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    max-width: 460px;
  }
  .auth-row {
    display: grid;
    grid-template-columns: 90px 1fr;
    align-items: center;
    gap: var(--space-3);
    font-size: var(--font-size-md);
  }
  .auth-row select,
  .field {
    background: var(--bg-elev);
    border: 1px solid var(--border);
    padding: var(--space-2) var(--space-3);
    outline: none;
    font-size: var(--font-size-md);
  }
  .auth-row select:focus,
  .field:focus {
    border-color: var(--accent);
  }
  .auth-row select option {
    background: var(--bg-elev-2);
    color: var(--text);
  }
  .auth-hint {
    margin: 0;
    color: var(--text-dim);
    font-size: var(--font-size-sm);
  }
</style>
