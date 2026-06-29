// capacitorBackend is the Android (Capacitor WebView) implementation of the
// Backend interface. getBackend() selects it when running inside a Capacitor
// shell (window.Capacitor present). It bridges every Backend call to the shared
// Go core compiled to an Android library (volt.aar via gomobile bind) through
// the custom VoltBridge Capacitor plugin (see ../native/voltBridgePlugin.ts and
// the native Kotlin plugin in frontend/native-android/VoltBridgePlugin.kt).
//
// Because requests run through native Go rather than the WebView's fetch, they
// are not subject to browser CORS (Req 15.2); the same httpcore engine the
// desktop build uses guarantees identical preparation/execution (Req 15.1); the
// engine surfaces network/validation failures in HTTPResponse.error so they are
// preserved and rendered like any other response (Req 15.4); and the Bridge's
// on-device SQLite store persists Collections, Environments, and History across
// restarts (Req 15.5).
//
// JSON convention (mirrors mobile/bind.go): every plugin method returns a
// `{ result }` string. For structured data the string is the model JSON on
// success or a `{"error":"..."}` envelope on failure; action-only methods
// return `{"ok":true}` or an error envelope; execute returns a JSON
// HTTPResponse whose own `error` field is part of the payload (not an envelope)
// and is therefore never thrown.

import type { Backend } from './index'
import type {
  Collection,
  Environment,
  Folder,
  HTTPResponse,
  HistoryEntry,
  ImportResult,
  RawRequest,
  SavedRequest,
  Settings,
} from '../models'
import { VoltBridge } from '../native/voltBridgePlugin'

/** The `{"error":"..."}` envelope shape the Bridge returns on CRUD failure. */
interface ErrorEnvelope {
  error: string
}

/** Type guard: true when a parsed value is a non-array object. */
function isObject(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

/** True when a parsed value is a Bridge `{"error":"..."}` failure envelope. */
function isErrorEnvelope(v: unknown): v is ErrorEnvelope {
  return isObject(v) && typeof v.error === 'string'
}

/**
 * Parses a Bridge JSON string that carries structured data on success or an
 * `{"error":"..."}` envelope on failure. Throws the Bridge error message so the
 * frontend's existing error handling renders it; otherwise returns the value
 * typed as T. CRUD/list results (Collection, Folder, arrays, ImportResult,
 * Settings, ...) never have a top-level string `error`, so the envelope check
 * is unambiguous.
 */
function parseResult<T>(method: string, raw: string): T {
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch (e) {
    throw new Error(`volt: capacitorBackend.${method} received malformed JSON from native bridge: ${String(e)}`)
  }
  if (isErrorEnvelope(parsed)) {
    throw new Error(parsed.error)
  }
  return parsed as T
}

/**
 * Parses the response of an action-only Bridge method, which returns
 * `{"ok":true}` on success or an `{"error":"..."}` envelope on failure. Throws
 * on the error envelope; resolves to void on success.
 */
function parseVoid(method: string, raw: string): void {
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch (e) {
    throw new Error(`volt: capacitorBackend.${method} received malformed JSON from native bridge: ${String(e)}`)
  }
  if (isErrorEnvelope(parsed)) {
    throw new Error(parsed.error)
  }
}

export const capacitorBackend: Backend = {
  // Request execution. The Bridge loads the active environment + settings from
  // its own store (matching the desktop ExecuteRequest path), so only the raw
  // request is marshalled. The returned HTTPResponse is parsed and returned as
  // is — its `error` field is a legitimate part of the payload (Req 15.4), not
  // a failure envelope, so it is not thrown.
  async executeRequest(req: RawRequest): Promise<HTTPResponse> {
    const { result } = await VoltBridge.execute({ reqJSON: JSON.stringify(req) })
    // NB: do not route through parseResult — an HTTPResponse always carries an
    // `error` field (empty on success), which is part of the payload, not a
    // failure envelope (Req 15.4). Parse it directly.
    let parsed: unknown
    try {
      parsed = JSON.parse(result)
    } catch (e) {
      throw new Error(`volt: capacitorBackend.executeRequest received malformed JSON from native bridge: ${String(e)}`)
    }
    return parsed as HTTPResponse
  },

  // Collection / Folder / Request tree
  async listTree(): Promise<Collection[]> {
    const { result } = await VoltBridge.listTree()
    return parseResult<Collection[]>('listTree', result)
  },
  async saveCollection(c: Collection): Promise<Collection> {
    const { result } = await VoltBridge.saveCollection({ collectionJSON: JSON.stringify(c) })
    return parseResult<Collection>('saveCollection', result)
  },
  async renameCollection(id: string, name: string): Promise<void> {
    const { result } = await VoltBridge.renameCollection({ id, name })
    parseVoid('renameCollection', result)
  },
  async deleteCollection(id: string): Promise<void> {
    const { result } = await VoltBridge.deleteCollection({ id })
    parseVoid('deleteCollection', result)
  },
  async saveFolder(f: Folder, parentId: string): Promise<Folder> {
    const { result } = await VoltBridge.saveFolder({ folderJSON: JSON.stringify(f), parentID: parentId })
    return parseResult<Folder>('saveFolder', result)
  },
  async deleteFolder(id: string): Promise<void> {
    const { result } = await VoltBridge.deleteFolder({ id })
    parseVoid('deleteFolder', result)
  },
  async saveRequest(req: SavedRequest, parentId: string): Promise<SavedRequest> {
    const { result } = await VoltBridge.saveRequest({ requestJSON: JSON.stringify(req), parentID: parentId })
    return parseResult<SavedRequest>('saveRequest', result)
  },
  async deleteRequest(id: string): Promise<void> {
    const { result } = await VoltBridge.deleteRequest({ id })
    parseVoid('deleteRequest', result)
  },
  async moveRequest(requestId: string, targetParentId: string): Promise<void> {
    const { result } = await VoltBridge.moveRequest({ requestID: requestId, targetParentID: targetParentId })
    parseVoid('moveRequest', result)
  },

  // Environments
  async listEnvironments(): Promise<Environment[]> {
    const { result } = await VoltBridge.listEnvironments()
    return parseResult<Environment[]>('listEnvironments', result)
  },
  async saveEnvironment(e: Environment): Promise<Environment> {
    const { result } = await VoltBridge.saveEnvironment({ environmentJSON: JSON.stringify(e) })
    return parseResult<Environment>('saveEnvironment', result)
  },
  async deleteEnvironment(id: string): Promise<void> {
    const { result } = await VoltBridge.deleteEnvironment({ id })
    parseVoid('deleteEnvironment', result)
  },
  async setActiveEnvironment(id: string): Promise<void> {
    const { result } = await VoltBridge.setActiveEnvironment({ id })
    parseVoid('setActiveEnvironment', result)
  },

  // History
  async listHistory(): Promise<HistoryEntry[]> {
    const { result } = await VoltBridge.listHistory()
    return parseResult<HistoryEntry[]>('listHistory', result)
  },
  async clearHistory(): Promise<void> {
    const { result } = await VoltBridge.clearHistory()
    parseVoid('clearHistory', result)
  },
  async migrateLegacyHistory(entries: HistoryEntry[]): Promise<boolean> {
    const { result } = await VoltBridge.migrateLegacyHistory({ entriesJSON: JSON.stringify(entries) })
    return parseResult<{ migrated: boolean }>('migrateLegacyHistory', result).migrated
  },

  // Settings
  async getSettings(): Promise<Settings> {
    const { result } = await VoltBridge.getSettings()
    return parseResult<Settings>('getSettings', result)
  },
  async saveSettings(s: Settings): Promise<void> {
    const { result } = await VoltBridge.saveSettings({ settingsJSON: JSON.stringify(s) })
    parseVoid('saveSettings', result)
  },

  // Import / Export. Export methods return the export JSON itself on success or
  // an error envelope on failure; the raw JSON string is returned to the caller
  // (matching wailsBackend, where []byte resolves to a string).
  async exportCollection(id: string): Promise<string> {
    const { result } = await VoltBridge.exportCollection({ id })
    return unwrapExport(result)
  },
  async exportEnvironment(id: string): Promise<string> {
    const { result } = await VoltBridge.exportEnvironment({ id })
    return unwrapExport(result)
  },
  async importData(data: string): Promise<ImportResult> {
    const { result } = await VoltBridge.importData({ dataJSON: data })
    return parseResult<ImportResult>('importData', result)
  },

  // Metadata. The native plugin returns BuildConfig.versionName as the plain
  // result string (no JSON envelope), matching wailsBackend.appVersion().
  async appVersion(): Promise<string> {
    const { result } = await VoltBridge.appVersion()
    return result
  },
}

/**
 * Unwraps an export method's result: export methods return the export JSON
 * itself on success, or a `{"error":"..."}` envelope on failure. On the error
 * envelope this throws the Bridge message; otherwise the raw JSON string is
 * returned to the caller (matching wailsBackend, where []byte resolves to a
 * base64/JSON string the caller forwards on).
 */
function unwrapExport(raw: string): string {
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    // Not valid JSON at all — return as is and let the caller deal with it.
    return raw
  }
  if (isErrorEnvelope(parsed)) {
    throw new Error(parsed.error)
  }
  return raw
}
