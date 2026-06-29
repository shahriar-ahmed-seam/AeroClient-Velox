<script lang="ts">
  // SettingsView — the Settings interface (Requirement 9). It exposes:
  //   - Theme selection (Light/Dark/System) applied live via settingsStore.setTheme
  //     (Req 9.1, 9.2). The store applies the resolved theme to the document root
  //     on save, so the change takes effect across all views without a restart.
  //   - TLS verification toggle via settingsStore.setTlsVerify (Req 9.1). While
  //     verification is disabled a persistent warning banner is shown (Req 9.4).
  //   - A request-timeout input in seconds. settingsStore.setTimeout validates the
  //     value is an integer in 1..600 and, on a rejected value, retains the previous
  //     timeout and surfaces a message via uiStore (Req 9.6); this view reflects that
  //     validation feedback and snaps the field back to the retained value.
  //   - A read-only listing of the global keyboard shortcuts (Req 10.6).
  //
  // All persistence is delegated to settingsStore (which routes through the
  // Backend); this component owns only its local view state. Styling uses the
  // design-system tokens and inherits the global border-radius:0 rule.
  import { settingsStore, uiStore } from '../../lib/stores'
  import type { Theme } from '../../lib/models'

  const settings = $derived(settingsStore.settings)

  /** Theme options shown in the selector, in display order. */
  const themeOptions: { value: Theme; label: string }[] = [
    { value: 'light', label: 'Light' },
    { value: 'dark', label: 'Dark' },
    { value: 'system', label: 'System' },
  ]

  // Local, editable copy of the timeout field. It mirrors the persisted value but
  // can hold a transient (possibly invalid) entry while the user types. It is
  // re-synced to the persisted value whenever settings change (including when an
  // invalid submission is rejected and the previous value is retained).
  let timeoutInput = $state(String(settingsStore.settings.timeoutSeconds))
  $effect(() => {
    timeoutInput = String(settings.timeoutSeconds)
  })

  function onTheme(value: Theme): void {
    void settingsStore.setTheme(value)
  }

  function onTlsVerify(checked: boolean): void {
    void settingsStore.setTlsVerify(checked)
  }

  // Commit the timeout entry. The store validates 1..600 and surfaces an error on
  // rejection while keeping the previous value; we then resync the field via the
  // $effect above so a rejected entry visibly snaps back to the retained value.
  async function commitTimeout(): Promise<void> {
    uiStore.clearError()
    const parsed = Number(timeoutInput.trim())
    await settingsStore.setTimeout(parsed)
    // Re-sync to the persisted value (unchanged if the entry was rejected).
    timeoutInput = String(settingsStore.settings.timeoutSeconds)
  }

  function onTimeoutKeydown(event: KeyboardEvent): void {
    if (event.key === 'Enter') {
      event.preventDefault()
      void commitTimeout()
    }
  }

  /** True on macOS, so shortcut chords display the Command key instead of Ctrl. */
  const isMac =
    typeof navigator !== 'undefined' && /Mac|iPhone|iPad/i.test(navigator.platform ?? '')
  const modKey = isMac ? 'Cmd' : 'Ctrl'

  /** The global keyboard shortcuts surfaced to the user (Req 10.6). */
  const shortcuts = $derived([
    { keys: `${modKey}+Enter`, command: 'Send request' },
    { keys: `${modKey}+K`, command: 'Open command palette' },
    { keys: `${modKey}+S`, command: 'Save request' },
  ])
</script>

<section class="settings" aria-label="Settings">
  <h1 class="settings-title">Settings</h1>

  {#if !settings.tlsVerify}
    <!-- Persistent warning while TLS verification is disabled (Req 9.4). -->
    <div class="tls-warning" role="alert">
      <span class="tls-warning-icon" aria-hidden="true">⚠</span>
      <span>
        TLS certificate verification is disabled. Requests will not validate server
        certificates, leaving connections vulnerable to interception.
      </span>
    </div>
  {/if}

  <!-- Theme (Req 9.1, 9.2) -->
  <fieldset class="group">
    <legend class="group-title">Appearance</legend>
    <label class="row">
      <span class="row-label">Theme</span>
      <select
        class="control"
        value={settings.theme}
        onchange={(e) => onTheme(e.currentTarget.value as Theme)}
      >
        {#each themeOptions as opt (opt.value)}
          <option value={opt.value}>{opt.label}</option>
        {/each}
      </select>
    </label>
  </fieldset>

  <!-- Network (TLS + timeout) (Req 9.1, 9.4, 9.6) -->
  <fieldset class="group">
    <legend class="group-title">Network</legend>

    <label class="row">
      <span class="row-label">Verify TLS certificates</span>
      <span class="control toggle-control">
        <input
          type="checkbox"
          checked={settings.tlsVerify}
          onchange={(e) => onTlsVerify(e.currentTarget.checked)}
        />
        <span class="toggle-hint">{settings.tlsVerify ? 'Enabled' : 'Disabled'}</span>
      </span>
    </label>

    <label class="row">
      <span class="row-label">Request timeout (seconds)</span>
      <input
        class="control field"
        type="number"
        min="1"
        max="600"
        step="1"
        inputmode="numeric"
        bind:value={timeoutInput}
        onblur={commitTimeout}
        onkeydown={onTimeoutKeydown}
      />
    </label>
    <p class="hint">Enter a whole number between 1 and 600 seconds.</p>

    {#if uiStore.errorMessage}
      <p class="invalid" role="alert">{uiStore.errorMessage}</p>
    {/if}
  </fieldset>

  <!-- Keyboard shortcuts listing (Req 10.6) -->
  <fieldset class="group">
    <legend class="group-title">Keyboard shortcuts</legend>
    <ul class="shortcuts">
      {#each shortcuts as sc (sc.keys)}
        <li class="shortcut">
          <kbd class="keys">{sc.keys}</kbd>
          <span class="shortcut-command">{sc.command}</span>
        </li>
      {/each}
    </ul>
  </fieldset>
</section>

<style>
  .settings {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    padding: var(--container-padding);
    max-width: 640px;
    color: var(--text);
    font-family: var(--font-sans);
  }

  .settings-title {
    margin: 0;
    font-size: var(--font-size-xl);
    font-weight: var(--font-weight-semibold);
    line-height: var(--line-height-tight);
  }

  .tls-warning {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background: var(--bg-elev-2);
    border: 1px solid var(--red);
    color: var(--text);
    font-size: var(--font-size-sm);
    line-height: var(--line-height-normal);
  }
  .tls-warning-icon {
    color: var(--red);
    font-size: var(--font-size-lg);
    line-height: 1;
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    margin: 0;
    padding: var(--space-4);
    background: var(--bg-elev);
    border: 1px solid var(--border);
  }
  .group-title {
    padding: 0 var(--space-2);
    color: var(--text-dim);
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .row {
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: var(--space-4);
    font-size: var(--font-size-md);
  }
  .row-label {
    color: var(--text);
  }

  .control {
    min-width: 160px;
  }
  .control.field,
  select.control {
    background: var(--bg-elev-2);
    border: 1px solid var(--border);
    color: var(--text);
    padding: var(--space-2) var(--space-3);
    outline: none;
    font-size: var(--font-size-md);
    font-family: var(--font-sans);
  }
  .control.field:focus,
  select.control:focus {
    border-color: var(--accent);
  }
  select.control option {
    background: var(--bg-elev-2);
    color: var(--text);
  }

  .toggle-control {
    display: inline-flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-2);
  }
  .toggle-control input[type='checkbox'] {
    width: var(--space-4);
    height: var(--space-4);
    accent-color: var(--accent);
  }
  .toggle-hint {
    color: var(--text-dim);
    font-size: var(--font-size-sm);
  }

  .hint {
    margin: 0;
    color: var(--text-dim);
    font-size: var(--font-size-sm);
  }
  .invalid {
    margin: 0;
    color: var(--red);
    font-size: var(--font-size-sm);
  }

  .shortcuts {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .shortcut {
    display: grid;
    grid-template-columns: 140px 1fr;
    align-items: center;
    gap: var(--space-4);
    font-size: var(--font-size-md);
  }
  .keys {
    justify-self: start;
    padding: var(--space-1) var(--space-2);
    background: var(--bg-elev-2);
    border: 1px solid var(--border-strong);
    color: var(--text);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
  }
  .shortcut-command {
    color: var(--text-dim);
  }
</style>
