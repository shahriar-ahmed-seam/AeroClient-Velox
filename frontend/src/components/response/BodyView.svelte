<script lang="ts">
  // BodyView renders the response body in one of three exclusive modes:
  //
  //   Pretty  — valid JSON is pretty-printed (2-space indent) with lightweight
  //             syntax highlighting; non-JSON falls back to unmodified text with
  //             no highlighting (Req 4.4, 4.5).
  //   Raw     — the body exactly as received (Req 4.3).
  //   Preview — when the response Content-Type is text/html the body is rendered
  //             inside a sandboxed <iframe> (scripts disabled) via srcdoc; any
  //             other content type shows the unmodified text (Req 4.6, 4.7).
  //
  // A copy control copies whatever text is currently displayed and shows a
  // transient confirmation (Req 4.12). When the engine flags the body truncated
  // (>5 MB) a banner plus a save control are shown (Req 4.11).
  import { prettyMaybeJSON } from '../../lib/types'
  import type { HTTPResponse, KeyValue } from '../../lib/models'

  let { response }: { response: HTTPResponse } = $props()

  type Mode = 'pretty' | 'raw' | 'preview'
  let mode = $state<Mode>('pretty')
  let copied = $state(false)
  let copyError = $state(false)
  let copyTimer: ReturnType<typeof setTimeout> | undefined

  // --- content-type detection (case-insensitive) ---------------------------
  const contentType = $derived(headerValue(response.headers, 'content-type'))
  const isHtml = $derived(contentType.toLowerCase().includes('text/html'))

  // --- pretty/raw text ------------------------------------------------------
  const prettyText = $derived(prettyMaybeJSON(response.body))
  const isJson = $derived(prettyText !== response.body || looksLikeJson(response.body))

  // The text the copy control should place on the clipboard: the Pretty view
  // copies the pretty-printed text, the others copy the raw body.
  const displayedText = $derived(mode === 'pretty' ? prettyText : response.body)

  function headerValue(headers: KeyValue[], name: string): string {
    const lower = name.toLowerCase()
    const found = headers.find((h) => h.key.toLowerCase() === lower)
    return found ? found.value : ''
  }

  function looksLikeJson(body: string): boolean {
    const trimmed = body.trim()
    if (trimmed === '') return false
    try {
      JSON.parse(trimmed)
      return true
    } catch {
      return false
    }
  }

  // --- JSON syntax highlighting --------------------------------------------
  // Escape first (so the body can never inject markup), then wrap JSON tokens
  // in classed spans. Returns escaped plain text untouched for non-JSON.
  function escapeHtml(s: string): string {
    return s
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
  }

  function highlightJson(s: string): string {
    const escaped = escapeHtml(s)
    return escaped.replace(
      /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g,
      (match) => {
        let cls = 'tok-num'
        if (/^"/.test(match)) {
          cls = /:$/.test(match) ? 'tok-key' : 'tok-str'
        } else if (/true|false/.test(match)) {
          cls = 'tok-bool'
        } else if (/null/.test(match)) {
          cls = 'tok-null'
        }
        return `<span class="${cls}">${match}</span>`
      },
    )
  }

  const prettyHtml = $derived(highlightJson(prettyText))

  // --- copy-to-clipboard ----------------------------------------------------
  async function copyBody(): Promise<void> {
    try {
      await navigator.clipboard.writeText(displayedText)
      copyError = false
      copied = true
    } catch {
      copyError = true
      copied = false
    }
    clearTimeout(copyTimer)
    copyTimer = setTimeout(() => {
      copied = false
      copyError = false
    }, 1500)
  }

  // --- save full body (truncation control) ----------------------------------
  function saveBody(): void {
    const blob = new Blob([response.body], {
      type: contentType || 'text/plain;charset=utf-8',
    })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'response-body.txt'
    a.click()
    URL.revokeObjectURL(url)
  }
</script>

<div class="bodyview">
  <div class="toolbar">
    <div class="modes" role="tablist" aria-label="Response body view">
      <button
        class="mode"
        class:active={mode === 'pretty'}
        role="tab"
        aria-selected={mode === 'pretty'}
        onclick={() => (mode = 'pretty')}
      >Pretty</button>
      <button
        class="mode"
        class:active={mode === 'raw'}
        role="tab"
        aria-selected={mode === 'raw'}
        onclick={() => (mode = 'raw')}
      >Raw</button>
      <button
        class="mode"
        class:active={mode === 'preview'}
        role="tab"
        aria-selected={mode === 'preview'}
        onclick={() => (mode = 'preview')}
      >Preview</button>
    </div>
    <div class="actions">
      <button class="action" onclick={copyBody}>
        {#if copied}Copied{:else if copyError}Copy failed{:else}Copy{/if}
      </button>
    </div>
  </div>

  {#if response.truncated}
    <div class="trunc" role="status">
      <span class="trunc-text">
        Response exceeds 5 MB — showing the first 5 MB only.
      </span>
      <button class="action" onclick={saveBody}>Save body</button>
    </div>
  {/if}

  <div class="content">
    {#if mode === 'preview'}
      {#if isHtml}
        <iframe
          class="preview-frame"
          title="Response preview"
          sandbox=""
          srcdoc={response.body}
        ></iframe>
      {:else}
        <pre class="text">{response.body}</pre>
      {/if}
    {:else if mode === 'raw'}
      <pre class="text">{response.body}</pre>
    {:else if isJson}
      <pre class="text json">{@html prettyHtml}</pre>
    {:else}
      <pre class="text">{prettyText}</pre>
    {/if}
  </div>
</div>

<style>
  .bodyview {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    border-bottom: 1px solid var(--border);
  }
  .modes { display: flex; gap: var(--space-1); }
  .mode {
    background: transparent;
    border: 1px solid transparent;
    color: var(--text-dim);
    padding: var(--space-1) var(--space-3);
    cursor: pointer;
    font-size: var(--font-size-sm);
  }
  .mode:hover { color: var(--text); }
  .mode.active {
    color: var(--text);
    border-color: var(--border-strong);
    background: var(--bg-elev-2);
  }
  .actions { display: flex; gap: var(--space-2); }
  .action {
    background: var(--bg-elev-2);
    border: 1px solid var(--border);
    color: var(--text-dim);
    font-size: var(--font-size-xs);
    padding: var(--space-1) var(--space-2);
    cursor: pointer;
  }
  .action:hover { color: var(--text); }

  .trunc {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-4);
    border-bottom: 1px solid var(--border);
    background: var(--bg-elev-2);
    color: var(--accent);
    font-size: var(--font-size-sm);
  }

  .content { flex: 1; min-height: 0; display: flex; }
  .text {
    flex: 1;
    margin: 0;
    overflow: auto;
    padding: var(--space-3) var(--space-4);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    line-height: var(--line-height-normal);
    white-space: pre-wrap;
    word-break: break-word;
    user-select: text;
  }
  .preview-frame {
    flex: 1;
    width: 100%;
    border: none;
    background: var(--color-text); /* white-ish canvas for rendered HTML */
  }

  /* JSON token colors (drawn from design tokens) */
  .json :global(.tok-key) { color: var(--accent-2); }
  .json :global(.tok-str) { color: var(--green); }
  .json :global(.tok-num) { color: var(--blue); }
  .json :global(.tok-bool) { color: var(--purple); }
  .json :global(.tok-null) { color: var(--text-dim); }
</style>
