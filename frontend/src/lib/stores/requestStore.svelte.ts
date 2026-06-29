// requestStore owns the request currently being edited in the workbench, the
// most recent response, and the in-flight/loading flag. It is the single place
// the editor components read and mutate the working RawRequest.
//
// Sending a request is delegated entirely to the Backend (getBackend()) which
// forwards to the shared Go core; the core also records the History entry, so
// after a send we refresh historyStore from the Backend rather than fabricating
// a local entry. Failures are surfaced through uiStore without throwing, so a
// failed send never crashes the editor and the user's input is preserved.

import { getBackend } from '../backend'
import {
  emptyRawRequest,
  type AuthSpec,
  type BodySpec,
  type HTTPResponse,
  type HistoryEntry,
  type KeyValue,
  type Method,
  type RawRequest,
  type SavedRequest,
} from '../models'
import { deepClone } from './clone'
import { historyStore } from './historyStore.svelte'
import { uiStore } from './uiStore.svelte'

class RequestStore {
  /** The request being edited. Components bind to this for two-way editing. */
  current = $state<RawRequest>(emptyRawRequest())

  /** The most recent response, or null before any send / after a reset. */
  response = $state<HTTPResponse | null>(null)

  /** True while a request is in flight. */
  loading = $state(false)

  // -- field mutators -------------------------------------------------------

  setMethod(method: Method): void {
    this.current.method = method
  }

  setUrl(url: string): void {
    this.current.url = url
  }

  setParams(params: KeyValue[]): void {
    this.current.params = params
  }

  setHeaders(headers: KeyValue[]): void {
    this.current.headers = headers
  }

  setBody(body: BodySpec): void {
    this.current.body = body
  }

  setAuth(auth: AuthSpec): void {
    this.current.auth = auth
  }

  /** Replace the editor with a fresh, empty request. */
  reset(): void {
    this.current = emptyRawRequest()
    this.response = null
  }

  // -- configuration restore (Req 7.4) -------------------------------------

  /**
   * Load a configuration into the editor. Accepts a bare RawRequest, a
   * SavedRequest (from the collections tree), or a HistoryEntry (whose `.request`
   * holds the full configuration). The configuration is deep-cloned so editing
   * the working copy never mutates the persisted source.
   *
   * Satisfies Requirement 7.4 (selecting a History entry restores the method,
   * URL, params, headers, body, and authorization into the editor) and the same
   * restore path for saved requests.
   */
  loadConfig(source: RawRequest | SavedRequest | HistoryEntry): void {
    const raw = isHistoryEntry(source) ? source.request : source
    this.current = deepClone(extractRawRequest(raw))
  }

  // -- execution ------------------------------------------------------------

  /**
   * Send the current request through the Backend and store the response. The
   * Backend records History on success and failure, so we reload historyStore
   * afterward. Any failure is surfaced via uiStore; the editor state is left
   * intact so the user can retry.
   */
  async send(): Promise<void> {
    if (this.loading) return
    this.loading = true
    uiStore.clearError()
    try {
      const res = await getBackend().executeRequest($state.snapshot(this.current))
      this.response = res
      // The Backend recorded the History entry as part of executing; pull the
      // authoritative list back rather than constructing an entry locally.
      await historyStore.add()
    } catch (err) {
      uiStore.showError(err)
    } finally {
      this.loading = false
    }
  }
}

/** Narrow a restore source to a HistoryEntry by its discriminating fields. */
function isHistoryEntry(value: RawRequest | SavedRequest | HistoryEntry): value is HistoryEntry {
  return (value as HistoryEntry).request !== undefined && (value as HistoryEntry).at !== undefined
}

/** Project any RawRequest-shaped value down to a plain RawRequest. */
function extractRawRequest(r: RawRequest): RawRequest {
  return {
    method: r.method,
    url: r.url,
    params: r.params,
    headers: r.headers,
    body: r.body,
    auth: r.auth,
  }
}

export const requestStore = new RequestStore()
