// The Backend abstraction the Svelte UI talks to. The frontend never calls the
// Go core directly; it depends only on this interface. Two implementations
// exist — wailsBackend (desktop) and capacitorBackend (Android) — both
// forwarding to the same shared Go core, so the UI code is identical across
// platforms. getBackend() selects the right one at startup.

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
import { wailsBackend } from './wails'
import { capacitorBackend } from './capacitor'

/**
 * Backend mirrors the App bindings exposed by the Go layer. Methods returning
 * Go `[]byte` (the export methods) resolve to a base64-encoded string, matching
 * how the bindings JSON-marshal byte slices.
 */
export interface Backend {
  // Request execution
  executeRequest(req: RawRequest): Promise<HTTPResponse>

  // Collection / Folder / Request tree
  listTree(): Promise<Collection[]>
  saveCollection(c: Collection): Promise<Collection>
  renameCollection(id: string, name: string): Promise<void>
  deleteCollection(id: string): Promise<void>
  saveFolder(f: Folder, parentId: string): Promise<Folder>
  deleteFolder(id: string): Promise<void>
  saveRequest(req: SavedRequest, parentId: string): Promise<SavedRequest>
  deleteRequest(id: string): Promise<void>
  moveRequest(requestId: string, targetParentId: string): Promise<void>

  // Environments
  listEnvironments(): Promise<Environment[]>
  saveEnvironment(e: Environment): Promise<Environment>
  deleteEnvironment(id: string): Promise<void>
  setActiveEnvironment(id: string): Promise<void>

  // History
  listHistory(): Promise<HistoryEntry[]>
  clearHistory(): Promise<void>
  migrateLegacyHistory(entries: HistoryEntry[]): Promise<boolean>

  // Settings
  getSettings(): Promise<Settings>
  saveSettings(s: Settings): Promise<void>

  // Import / Export
  exportCollection(id: string): Promise<string>
  exportEnvironment(id: string): Promise<string>
  importData(data: string): Promise<ImportResult>

  // Metadata
  appVersion(): Promise<string>
}

/** True when running inside a Capacitor (Android) WebView shell. */
function isCapacitor(): boolean {
  return typeof window !== 'undefined' && (window as { Capacitor?: unknown }).Capacitor != null
}

let selected: Backend | null = null

/**
 * Returns the platform Backend: capacitorBackend under Capacitor, otherwise the
 * Wails desktop backend. The choice is made once and cached.
 */
export function getBackend(): Backend {
  if (selected == null) {
    selected = isCapacitor() ? capacitorBackend : wailsBackend
  }
  return selected
}

export { wailsBackend } from './wails'
export { capacitorBackend } from './capacitor'
