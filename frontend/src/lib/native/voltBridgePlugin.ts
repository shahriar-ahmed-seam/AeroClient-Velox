// The Volt custom Capacitor plugin definition + registration.
//
// This module declares the JavaScript-facing contract of the native Android
// plugin ("VoltBridge") and registers it with Capacitor. The plugin is the
// JS↔native bridge that lets the WebView call the shared Go core compiled into
// an Android library (`volt.aar` via `gomobile bind`, exposing `mobile.Bridge`;
// see mobile/bind.go). Because requests run through native Go rather than the
// WebView's `fetch`, Android requests are not subject to browser CORS
// (Req 15.2), and persistence is handled by the on-device SQLite store the
// Bridge owns (Req 15.5).
//
// Design reference: design.md, "Resolving the Android Open Technical Decision"
// and the Bindings section.
//
// gomobile bind supports only a restricted type set, so `mobile.Bridge` uses a
// string-in/string-out (JSON) convention for all structured data. The native
// Kotlin plugin (see frontend/native-android/VoltBridgePlugin.kt) forwards each
// call to the matching Bridge method and returns its JSON string verbatim. To
// keep the Capacitor calling convention uniform, every plugin method here:
//   - takes a single options object whose fields are plain strings, and
//   - resolves to `{ result: string }`, where `result` is the JSON string the
//     Bridge returned (or, for `appVersion`, the plain version string).
//
// `registerPlugin` is safe to call at import time on every platform: it returns
// a lazy proxy and instantiates nothing. On web/desktop (where there is no
// native VoltBridge and `window.Capacitor` is absent) this module is imported
// but never invoked, because getBackend() only selects capacitorBackend when
// running inside a Capacitor WebView. Invoking a method on a platform with no
// implementation rejects with a Capacitor "not implemented" error, which is the
// correct, clearly-surfaced behavior.

import { registerPlugin } from '@capacitor/core'

/** The uniform success shape returned by every Bridge-backed plugin method. */
export interface BridgeResult {
  /**
   * The JSON string returned by the corresponding `mobile.Bridge` method, or a
   * plain version string for `appVersion`. CRUD methods return either the
   * result JSON or a `{"error":"..."}` envelope; action-only methods return
   * `{"ok":true}` or an error envelope; `execute` returns a JSON
   * `model.HTTPResponse` (whose own `error` field carries network/validation
   * failures, Req 15.4).
   */
  result: string
}

/**
 * VoltBridgePlugin mirrors the methods of the Go `mobile.Bridge` facade
 * one-for-one. Argument and return payloads are JSON strings, matching the
 * binding's string-in/string-out convention. The native implementation
 * constructs a single `mobile.Bridge` per session (pointed at the app's
 * on-device database path) and forwards each call.
 */
export interface VoltBridgePlugin {
  // Request execution -------------------------------------------------------
  /** Forwards to Bridge.Execute(reqJSON). Resolves to a JSON HTTPResponse. */
  execute(options: { reqJSON: string }): Promise<BridgeResult>

  // Collections -------------------------------------------------------------
  saveCollection(options: { collectionJSON: string }): Promise<BridgeResult>
  renameCollection(options: { id: string; name: string }): Promise<BridgeResult>
  deleteCollection(options: { id: string }): Promise<BridgeResult>

  // Folders -----------------------------------------------------------------
  saveFolder(options: { folderJSON: string; parentID: string }): Promise<BridgeResult>
  deleteFolder(options: { id: string }): Promise<BridgeResult>

  // Requests ----------------------------------------------------------------
  saveRequest(options: { requestJSON: string; parentID: string }): Promise<BridgeResult>
  deleteRequest(options: { id: string }): Promise<BridgeResult>
  moveRequest(options: { requestID: string; targetParentID: string }): Promise<BridgeResult>
  listTree(): Promise<BridgeResult>

  // Environments ------------------------------------------------------------
  saveEnvironment(options: { environmentJSON: string }): Promise<BridgeResult>
  deleteEnvironment(options: { id: string }): Promise<BridgeResult>
  setActiveEnvironment(options: { id: string }): Promise<BridgeResult>
  listEnvironments(): Promise<BridgeResult>

  // History -----------------------------------------------------------------
  addHistory(options: { entryJSON: string }): Promise<BridgeResult>
  listHistory(): Promise<BridgeResult>
  clearHistory(): Promise<BridgeResult>
  migrateLegacyHistory(options: { entriesJSON: string }): Promise<BridgeResult>

  // Settings ----------------------------------------------------------------
  getSettings(): Promise<BridgeResult>
  saveSettings(options: { settingsJSON: string }): Promise<BridgeResult>

  // Import / Export ---------------------------------------------------------
  exportCollection(options: { id: string }): Promise<BridgeResult>
  exportEnvironment(options: { id: string }): Promise<BridgeResult>
  importData(options: { dataJSON: string }): Promise<BridgeResult>

  // Metadata ----------------------------------------------------------------
  /**
   * Returns the running app's Semantic_Version. Unlike the other methods this
   * has no Bridge counterpart: the native plugin returns `BuildConfig`'s
   * `versionName` (stamped by scripts/set-android-version.mjs), so the in-app
   * About/Settings view shows the same version baked into the APK (Req 15.5).
   */
  appVersion(): Promise<BridgeResult>
}

/**
 * The registered plugin instance. `registerPlugin` returns a proxy bound to the
 * native "VoltBridge" plugin at runtime; no native code is loaded at import, so
 * importing this module is safe in the web/desktop build and under tests.
 */
export const VoltBridge = registerPlugin<VoltBridgePlugin>('VoltBridge')
