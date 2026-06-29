// @vitest-environment jsdom
//
// Unit tests for the response viewer branches (task 15.4).
//
// ResponseViewer is a store-driven container: it reads requestStore.response,
// requestStore.loading, and uiStore.activeResponseTab and chooses exactly one
// of four render branches (loading → empty → error → status+tabs). Its Body tab
// embeds BodyView, whose Preview mode renders an <iframe> for text/html bodies
// and plain text otherwise, and whose Copy control writes the displayed text to
// the clipboard and flips to a transient "Copied" confirmation.
//
// Because the stores are module-level singletons, each test drives a branch by
// setting the public $state fields directly (requestStore.response / .loading,
// uiStore.activeResponseTab) before rendering, and resets them afterward so the
// tests stay independent.
//
// Validates: Requirements 4.3, 4.6, 4.7, 4.9, 4.10, 4.12

import {describe, it, expect, afterEach, beforeEach, vi} from 'vitest'
import {render, cleanup, fireEvent} from '@testing-library/svelte'
import type {HTTPResponse, KeyValue} from '../../lib/models'
import {requestStore, uiStore} from '../../lib/stores'
import ResponseViewer from './ResponseViewer.svelte'

// Build an otherwise-successful HTTPResponse, overriding only the fields a test
// cares about. Defaults describe a small 200 OK text/plain body so the
// status+tabs branch renders cleanly.
function makeResponse(overrides: Partial<HTTPResponse> = {}): HTTPResponse {
  const defaultHeaders: KeyValue[] = [
    {key: 'Content-Type', value: 'text/plain', enabled: true},
  ]
  return {
    status: 200,
    statusText: 'OK',
    headers: defaultHeaders,
    body: 'hello world',
    durationMs: 12,
    sizeBytes: 11,
    error: '',
    truncated: false,
    ...overrides,
  }
}

// Reset the shared store state before and after every test so each branch is
// driven from a known baseline and nothing leaks between cases.
function resetStores(): void {
  requestStore.response = null
  requestStore.loading = false
  uiStore.activeResponseTab = 'body'
}

beforeEach(() => {
  resetStores()
})

afterEach(() => {
  cleanup()
  resetStores()
  vi.restoreAllMocks()
})

describe('ResponseViewer render branches', () => {
  it('shows the loading indicator while a request is in flight (Req 4.10)', () => {
    requestStore.loading = true
    requestStore.response = null

    const {getByRole, getByText} = render(ResponseViewer)

    // The loading branch renders a role="status" region with the sending copy.
    expect(getByRole('status')).toBeTruthy()
    expect(getByText('Sending request…')).toBeTruthy()
  })

  it('loading takes precedence even when a response is present (Req 4.10)', () => {
    // A stale response plus loading=true must still show the loading branch,
    // never the response body, since loading has the highest precedence.
    requestStore.response = makeResponse()
    requestStore.loading = true

    const {getByText, queryByRole} = render(ResponseViewer)

    expect(getByText('Sending request…')).toBeTruthy()
    // No body/headers tablist should be present while loading.
    expect(queryByRole('tablist', {name: 'Response sections'})).toBeNull()
  })

  it('shows the empty state when there is no response and not loading', () => {
    requestStore.response = null
    requestStore.loading = false

    const {getByText} = render(ResponseViewer)

    expect(getByText('Send a request to see the response.')).toBeTruthy()
  })

  it('renders the error message and no status when response.error is set (Req 4.9)', () => {
    requestStore.response = makeResponse({
      error: 'dial tcp: connection refused',
      // The engine zeroes these on error; the viewer must not surface them.
      status: 0,
      statusText: '',
      durationMs: 0,
      sizeBytes: 0,
    })

    const {getByRole, getByText, queryByText} = render(ResponseViewer)

    // Error branch: an alert region with the failure label and message.
    expect(getByRole('alert')).toBeTruthy()
    expect(getByText('Request failed')).toBeTruthy()
    expect(getByText('dial tcp: connection refused')).toBeTruthy()
    // No status metrics are shown on the error branch.
    expect(queryByText('Time')).toBeNull()
    expect(queryByText('Size')).toBeNull()
  })

  it('toggles between Body and Headers tabs via uiStore.activeResponseTab (Req 4.3)', async () => {
    requestStore.response = makeResponse({
      headers: [
        {key: 'Content-Type', value: 'text/plain', enabled: true},
        {key: 'X-Trace', value: 'abc123', enabled: true},
      ],
      body: 'plain body text',
    })
    uiStore.activeResponseTab = 'body'

    const {getByRole, queryByText, getByText} = render(ResponseViewer)

    // Body tab active: BodyView's Pretty/Raw/Preview toolbar is present, and the
    // body text is shown; the Headers view's values are not.
    expect(getByRole('tablist', {name: 'Response body view'})).toBeTruthy()
    expect(getByText('plain body text')).toBeTruthy()
    expect(queryByText('abc123')).toBeNull()

    // Click the Headers tab → uiStore.setResponseTab('headers') flips the branch.
    await fireEvent.click(getByRole('tab', {name: /Headers/}))

    expect(uiStore.activeResponseTab).toBe('headers')
    // HeadersView now shows the header value; BodyView's toolbar is gone.
    expect(getByText('abc123')).toBeTruthy()
    expect(() => getByRole('tablist', {name: 'Response body view'})).toThrow()
  })

  it('renders an <iframe> preview for an HTML body in Preview mode (Req 4.6)', async () => {
    requestStore.response = makeResponse({
      headers: [{key: 'Content-Type', value: 'text/html; charset=utf-8', enabled: true}],
      body: '<h1>Hello</h1>',
    })
    uiStore.activeResponseTab = 'body'

    const {getByRole, container} = render(ResponseViewer)

    // Switch BodyView into Preview mode.
    await fireEvent.click(getByRole('tab', {name: 'Preview'}))

    const frame = container.querySelector('iframe.preview-frame') as HTMLIFrameElement | null
    expect(frame).not.toBeNull()
    // The HTML body is rendered through the sandboxed srcdoc, not as text.
    expect(frame?.getAttribute('srcdoc')).toBe('<h1>Hello</h1>')
  })

  it('shows the body as plain text (no iframe) for a non-previewable type in Preview mode (Req 4.7)', async () => {
    requestStore.response = makeResponse({
      headers: [{key: 'Content-Type', value: 'text/plain', enabled: true}],
      body: 'not html, just text',
    })
    uiStore.activeResponseTab = 'body'

    const {getByRole, getByText, container} = render(ResponseViewer)

    await fireEvent.click(getByRole('tab', {name: 'Preview'}))

    // A non-HTML content type renders the unmodified body as text, no iframe.
    expect(container.querySelector('iframe')).toBeNull()
    expect(getByText('not html, just text')).toBeTruthy()
  })

  it('copies the body and shows the "Copied" confirmation on Copy (Req 4.12)', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: {writeText},
      configurable: true,
    })

    requestStore.response = makeResponse({
      headers: [{key: 'Content-Type', value: 'text/plain', enabled: true}],
      body: 'copy me',
    })
    uiStore.activeResponseTab = 'body'

    const {getByText} = render(ResponseViewer)

    // The Copy control starts as "Copy".
    const copyButton = getByText('Copy')
    await fireEvent.click(copyButton)

    // The clipboard received the displayed text...
    expect(writeText).toHaveBeenCalledTimes(1)
    expect(writeText).toHaveBeenCalledWith('copy me')
    // ...and the control flips to its confirmation.
    expect(getByText('Copied')).toBeTruthy()
  })
})
