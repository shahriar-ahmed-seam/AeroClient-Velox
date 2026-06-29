<script lang="ts">
  // UnresolvedHint — a lightweight, non-blocking inline indication that one or
  // more variable-bearing fields contain `{{name}}` tokens that do not resolve
  // against the active environment (Requirement 6.6).
  //
  // Unresolved tokens are NOT an error: the engine still sends them literally
  // (Req 6.8). This component is purely a visual hint, so it never disables a
  // control or blocks Send. It renders nothing when every token resolves (or
  // when there are no tokens at all).
  //
  // It accepts one or more strings via `texts`; the distinct unresolved tokens
  // across all of them are gathered (in order of first appearance) using the
  // pure interpolationStore.findUnresolved logic, which mirrors the core's
  // token matching against the active environment's variables.
  import { interpolationStore } from '../../lib/stores'

  interface Props {
    // The variable-bearing field values to scan. A single field can be passed
    // as a one-element array; callers with several fields (e.g. all header
    // values) pass them together so the hint summarizes them as one line.
    texts: string[]
    // Optional label describing what was scanned, shown before the token list.
    label?: string
  }

  const { texts, label = 'Unresolved' }: Props = $props()

  // Distinct unresolved tokens across every provided field, preserving the
  // order of first appearance. Derived so it re-evaluates whenever the inputs
  // or the active environment's defined variables change.
  const unresolved = $derived.by<string[]>(() => {
    const seen = new Set<string>()
    const out: string[] = []
    for (const text of texts) {
      for (const token of interpolationStore.findUnresolved(text)) {
        if (!seen.has(token)) {
          seen.add(token)
          out.push(token)
        }
      }
    }
    return out
  })
</script>

{#if unresolved.length > 0}
  <p class="unresolved-hint" role="status">
    <span class="marker" aria-hidden="true">⚠</span>
    <span class="text">
      {label} {unresolved.length === 1 ? 'variable' : 'variables'}:
      {#each unresolved as token, i (token)}
        <code class="token">{token}</code>{#if i < unresolved.length - 1}<span class="sep">,</span>{/if}
      {/each}
      <span class="note">— sent literally</span>
    </span>
  </p>
{/if}

<style>
  .unresolved-hint {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    margin: 0;
    color: var(--accent);
    font-size: var(--font-size-sm);
    line-height: var(--line-height-normal);
  }
  .marker {
    flex: none;
    font-size: var(--font-size-xs);
  }
  .text {
    min-width: 0;
  }
  .token {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--accent);
    background: var(--bg-elev-2);
    padding: 0 var(--space-1);
  }
  .sep {
    margin-right: var(--space-1);
  }
  .note {
    color: var(--text-dim);
    margin-left: var(--space-1);
  }
</style>
