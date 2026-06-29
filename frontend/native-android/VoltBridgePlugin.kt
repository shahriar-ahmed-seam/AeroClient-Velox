// VoltBridgePlugin.kt — the native Android side of Volt's custom Capacitor
// plugin.
//
// This is a TEMPLATE. The native android/ project is generated on demand
// (`npx cap add android`) and is not committed, so this file lives under
// frontend/native-android/ and is copied into the generated project. See
// frontend/native-android/README.md for the exact copy + registration steps.
//
// What it does
// ------------
// It bridges the WebView's JS plugin calls (declared in
// src/lib/native/voltBridgePlugin.ts) to the shared Go core compiled into
// volt.aar by `scripts/build-aar.sh` (`gomobile bind ./mobile`). Each method
// forwards to the matching method on a single `mobile.Bridge` instance, so
// Android requests run through the exact same httpcore/store logic as the
// desktop build (Req 15.1), are not subject to WebView CORS because they go
// through native Go rather than fetch (Req 15.2), preserve engine errors in the
// returned HTTPResponse JSON (Req 15.4), and persist in the Bridge's on-device
// SQLite database (Req 15.5).
//
// Calling convention (matches mobile/bind.go and the TS plugin): each method
// receives string args from the JS `options` object and resolves a single
// `result` string — the JSON the Bridge returned (or BuildConfig.VERSION_NAME
// for appVersion). The JS layer parses/validates that JSON.
//
// IMPORTANT: adjust the import below to match the -javapkg passed to
// `gomobile bind`. The build script uses `-javapkg=dev.volt.apiclient.bridge`,
// which makes the generated package `dev.volt.apiclient.bridge.mobile` with a
// package class `Mobile` exposing `Mobile.newBridge(path)` and the `Bridge`
// type.

package dev.volt.apiclient

import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin

// gomobile-generated bindings from volt.aar (package set by -javapkg).
import dev.volt.apiclient.bridge.mobile.Bridge
import dev.volt.apiclient.bridge.mobile.Mobile

@CapacitorPlugin(name = "VoltBridge")
class VoltBridgePlugin : Plugin() {

    // A single Bridge per app session, opened lazily against the app's private
    // database file. Using the app's filesDir keeps the SQLite database in
    // app-private on-device storage (Req 15.5).
    private val bridge: Bridge by lazy {
        val dbPath = context.filesDir.resolve("volt.db").absolutePath
        // Mobile.newBridge returns the Bridge or throws on open failure.
        Mobile.newBridge(dbPath)
    }

    // --- Helpers -----------------------------------------------------------

    /** Resolves the call with the uniform { result } shape the JS layer expects. */
    private fun resolveResult(call: PluginCall, result: String) {
        val ret = com.getcapacitor.JSObject()
        ret.put("result", result)
        call.resolve(ret)
    }

    /** Reads a required string arg, rejecting the call if it is absent. */
    private fun requireArg(call: PluginCall, name: String): String? {
        val v = call.getString(name)
        if (v == null) {
            call.reject("missing required argument: $name")
        }
        return v
    }

    /**
     * Runs a Bridge call that needs no failure handling here (the Bridge encodes
     * its own errors into the returned JSON), guarding against unexpected native
     * exceptions so the JS promise always settles.
     */
    private fun forward(call: PluginCall, block: () -> String) {
        try {
            resolveResult(call, block())
        } catch (t: Throwable) {
            // Surface as the same JSON error envelope the Bridge uses, so the JS
            // layer's error handling path is identical.
            resolveResult(call, """{"error":${jsonString(t.message ?: "native bridge error")}}""")
        }
    }

    /** Minimal JSON string encoder for the rare native-exception fallback. */
    private fun jsonString(s: String): String {
        val sb = StringBuilder("\"")
        for (c in s) {
            when (c) {
                '\\' -> sb.append("\\\\")
                '"' -> sb.append("\\\"")
                '\n' -> sb.append("\\n")
                '\r' -> sb.append("\\r")
                '\t' -> sb.append("\\t")
                else -> if (c < ' ') sb.append("\\u%04x".format(c.code)) else sb.append(c)
            }
        }
        return sb.append("\"").toString()
    }

    // --- Request execution -------------------------------------------------

    @PluginMethod
    fun execute(call: PluginCall) {
        val reqJSON = requireArg(call, "reqJSON") ?: return
        forward(call) { bridge.execute(reqJSON) }
    }

    // --- Collections -------------------------------------------------------

    @PluginMethod
    fun saveCollection(call: PluginCall) {
        val collectionJSON = requireArg(call, "collectionJSON") ?: return
        forward(call) { bridge.saveCollection(collectionJSON) }
    }

    @PluginMethod
    fun renameCollection(call: PluginCall) {
        val id = requireArg(call, "id") ?: return
        val name = requireArg(call, "name") ?: return
        forward(call) { bridge.renameCollection(id, name) }
    }

    @PluginMethod
    fun deleteCollection(call: PluginCall) {
        val id = requireArg(call, "id") ?: return
        forward(call) { bridge.deleteCollection(id) }
    }

    // --- Folders -----------------------------------------------------------

    @PluginMethod
    fun saveFolder(call: PluginCall) {
        val folderJSON = requireArg(call, "folderJSON") ?: return
        val parentID = requireArg(call, "parentID") ?: return
        forward(call) { bridge.saveFolder(folderJSON, parentID) }
    }

    @PluginMethod
    fun deleteFolder(call: PluginCall) {
        val id = requireArg(call, "id") ?: return
        forward(call) { bridge.deleteFolder(id) }
    }

    // --- Requests ----------------------------------------------------------

    @PluginMethod
    fun saveRequest(call: PluginCall) {
        val requestJSON = requireArg(call, "requestJSON") ?: return
        val parentID = requireArg(call, "parentID") ?: return
        forward(call) { bridge.saveRequest(requestJSON, parentID) }
    }

    @PluginMethod
    fun deleteRequest(call: PluginCall) {
        val id = requireArg(call, "id") ?: return
        forward(call) { bridge.deleteRequest(id) }
    }

    @PluginMethod
    fun moveRequest(call: PluginCall) {
        val requestID = requireArg(call, "requestID") ?: return
        val targetParentID = requireArg(call, "targetParentID") ?: return
        forward(call) { bridge.moveRequest(requestID, targetParentID) }
    }

    @PluginMethod
    fun listTree(call: PluginCall) {
        forward(call) { bridge.listTree() }
    }

    // --- Environments ------------------------------------------------------

    @PluginMethod
    fun saveEnvironment(call: PluginCall) {
        val environmentJSON = requireArg(call, "environmentJSON") ?: return
        forward(call) { bridge.saveEnvironment(environmentJSON) }
    }

    @PluginMethod
    fun deleteEnvironment(call: PluginCall) {
        val id = requireArg(call, "id") ?: return
        forward(call) { bridge.deleteEnvironment(id) }
    }

    @PluginMethod
    fun setActiveEnvironment(call: PluginCall) {
        // id may be empty string to clear the active environment.
        val id = call.getString("id") ?: ""
        forward(call) { bridge.setActiveEnvironment(id) }
    }

    @PluginMethod
    fun listEnvironments(call: PluginCall) {
        forward(call) { bridge.listEnvironments() }
    }

    // --- History -----------------------------------------------------------

    @PluginMethod
    fun addHistory(call: PluginCall) {
        val entryJSON = requireArg(call, "entryJSON") ?: return
        forward(call) { bridge.addHistory(entryJSON) }
    }

    @PluginMethod
    fun listHistory(call: PluginCall) {
        forward(call) { bridge.listHistory() }
    }

    @PluginMethod
    fun clearHistory(call: PluginCall) {
        forward(call) { bridge.clearHistory() }
    }

    @PluginMethod
    fun migrateLegacyHistory(call: PluginCall) {
        val entriesJSON = requireArg(call, "entriesJSON") ?: return
        forward(call) { bridge.migrateLegacyHistory(entriesJSON) }
    }

    // --- Settings ----------------------------------------------------------

    @PluginMethod
    fun getSettings(call: PluginCall) {
        forward(call) { bridge.getSettings() }
    }

    @PluginMethod
    fun saveSettings(call: PluginCall) {
        val settingsJSON = requireArg(call, "settingsJSON") ?: return
        forward(call) { bridge.saveSettings(settingsJSON) }
    }

    // --- Import / Export ---------------------------------------------------

    @PluginMethod
    fun exportCollection(call: PluginCall) {
        val id = requireArg(call, "id") ?: return
        forward(call) { bridge.exportCollection(id) }
    }

    @PluginMethod
    fun exportEnvironment(call: PluginCall) {
        val id = requireArg(call, "id") ?: return
        forward(call) { bridge.exportEnvironment(id) }
    }

    @PluginMethod
    fun importData(call: PluginCall) {
        val dataJSON = requireArg(call, "dataJSON") ?: return
        forward(call) { bridge.import_(dataJSON) }
    }

    // --- Metadata ----------------------------------------------------------

    @PluginMethod
    fun appVersion(call: PluginCall) {
        // The version is the APK's versionName, stamped by
        // scripts/set-android-version.mjs before the release build (Req 15.5).
        resolveResult(call, BuildConfig.VERSION_NAME)
    }
}
