// VoltBridgePlugin.java — the native Android side of AeroClient-Velox's custom
// Capacitor plugin.
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
// This is the Java port of the original Kotlin template. Capacitor 6 generates
// a Java Android app by default; using Java here removes any dependency on the
// Kotlin toolchain being wired into the generated Gradle project.
//
// Calling convention (matches mobile/bind.go and the TS plugin): each method
// receives string args from the JS `options` object and resolves a single
// `result` string — the JSON the Bridge returned (or the APK versionName for
// appVersion). The JS layer parses/validates that JSON.
//
// The gomobile-generated bindings come from volt.aar, whose Java package is set
// by the build script's `-javapkg=dev.volt.apiclient.bridge`, producing the
// package `dev.volt.apiclient.bridge.mobile` with a package class `Mobile`
// exposing `Mobile.newBridge(path)` and the `Bridge` type.

package dev.volt.apiclient;

import android.content.pm.PackageManager;

import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;

// gomobile-generated bindings from volt.aar (package set by -javapkg).
import dev.volt.apiclient.bridge.mobile.Bridge;
import dev.volt.apiclient.bridge.mobile.Mobile;

@CapacitorPlugin(name = "VoltBridge")
public class VoltBridgePlugin extends Plugin {

    // A single Bridge per app session, opened lazily against the app's private
    // database file. Using the app's filesDir keeps the SQLite database in
    // app-private on-device storage (Req 15.5).
    private Bridge bridge;

    /**
     * Lazily opens (or returns) the Bridge against the app's private database
     * file. Mobile.newBridge throws on open failure.
     */
    private Bridge nativeBridge() throws Exception {
        if (bridge == null) {
            String dbPath = new java.io.File(getContext().getFilesDir(), "volt.db").getAbsolutePath();
            bridge = Mobile.newBridge(dbPath);
        }
        return bridge;
    }

    // --- Helpers -----------------------------------------------------------

    /** A Bridge call producing the result JSON string. */
    private interface BridgeCall {
        String run(Bridge b) throws Exception;
    }

    /** Resolves the call with the uniform { result } shape the JS layer expects. */
    private void resolveResult(PluginCall call, String result) {
        JSObject ret = new JSObject();
        ret.put("result", result);
        call.resolve(ret);
    }

    /** Reads a required string arg, rejecting the call (and returning null) if absent. */
    private String requireArg(PluginCall call, String name) {
        String v = call.getString(name);
        if (v == null) {
            call.reject("missing required argument: " + name);
        }
        return v;
    }

    /**
     * Runs a Bridge call that needs no failure handling here (the Bridge encodes
     * its own errors into the returned JSON), guarding against unexpected native
     * exceptions so the JS promise always settles.
     */
    private void forward(PluginCall call, BridgeCall block) {
        try {
            resolveResult(call, block.run(nativeBridge()));
        } catch (Throwable t) {
            // Surface as the same JSON error envelope the Bridge uses, so the JS
            // layer's error handling path is identical.
            String msg = t.getMessage() != null ? t.getMessage() : "native bridge error";
            resolveResult(call, "{\"error\":" + jsonString(msg) + "}");
        }
    }

    /** Minimal JSON string encoder for the rare native-exception fallback. */
    private String jsonString(String s) {
        StringBuilder sb = new StringBuilder("\"");
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            switch (c) {
                case '\\': sb.append("\\\\"); break;
                case '"': sb.append("\\\""); break;
                case '\n': sb.append("\\n"); break;
                case '\r': sb.append("\\r"); break;
                case '\t': sb.append("\\t"); break;
                default:
                    if (c < ' ') {
                        sb.append(String.format("\\u%04x", (int) c));
                    } else {
                        sb.append(c);
                    }
            }
        }
        return sb.append("\"").toString();
    }

    // --- Request execution -------------------------------------------------

    @PluginMethod
    public void execute(PluginCall call) {
        String reqJSON = requireArg(call, "reqJSON");
        if (reqJSON == null) return;
        forward(call, b -> b.execute(reqJSON));
    }

    // --- Collections -------------------------------------------------------

    @PluginMethod
    public void saveCollection(PluginCall call) {
        String collectionJSON = requireArg(call, "collectionJSON");
        if (collectionJSON == null) return;
        forward(call, b -> b.saveCollection(collectionJSON));
    }

    @PluginMethod
    public void renameCollection(PluginCall call) {
        String id = requireArg(call, "id");
        if (id == null) return;
        String name = requireArg(call, "name");
        if (name == null) return;
        forward(call, b -> b.renameCollection(id, name));
    }

    @PluginMethod
    public void deleteCollection(PluginCall call) {
        String id = requireArg(call, "id");
        if (id == null) return;
        forward(call, b -> b.deleteCollection(id));
    }

    // --- Folders -----------------------------------------------------------

    @PluginMethod
    public void saveFolder(PluginCall call) {
        String folderJSON = requireArg(call, "folderJSON");
        if (folderJSON == null) return;
        String parentID = requireArg(call, "parentID");
        if (parentID == null) return;
        forward(call, b -> b.saveFolder(folderJSON, parentID));
    }

    @PluginMethod
    public void deleteFolder(PluginCall call) {
        String id = requireArg(call, "id");
        if (id == null) return;
        forward(call, b -> b.deleteFolder(id));
    }

    // --- Requests ----------------------------------------------------------

    @PluginMethod
    public void saveRequest(PluginCall call) {
        String requestJSON = requireArg(call, "requestJSON");
        if (requestJSON == null) return;
        String parentID = requireArg(call, "parentID");
        if (parentID == null) return;
        forward(call, b -> b.saveRequest(requestJSON, parentID));
    }

    @PluginMethod
    public void deleteRequest(PluginCall call) {
        String id = requireArg(call, "id");
        if (id == null) return;
        forward(call, b -> b.deleteRequest(id));
    }

    @PluginMethod
    public void moveRequest(PluginCall call) {
        String requestID = requireArg(call, "requestID");
        if (requestID == null) return;
        String targetParentID = requireArg(call, "targetParentID");
        if (targetParentID == null) return;
        forward(call, b -> b.moveRequest(requestID, targetParentID));
    }

    @PluginMethod
    public void listTree(PluginCall call) {
        forward(call, Bridge::listTree);
    }

    // --- Environments ------------------------------------------------------

    @PluginMethod
    public void saveEnvironment(PluginCall call) {
        String environmentJSON = requireArg(call, "environmentJSON");
        if (environmentJSON == null) return;
        forward(call, b -> b.saveEnvironment(environmentJSON));
    }

    @PluginMethod
    public void deleteEnvironment(PluginCall call) {
        String id = requireArg(call, "id");
        if (id == null) return;
        forward(call, b -> b.deleteEnvironment(id));
    }

    @PluginMethod
    public void setActiveEnvironment(PluginCall call) {
        // id may be empty string to clear the active environment.
        String id = call.getString("id", "");
        forward(call, b -> b.setActiveEnvironment(id));
    }

    @PluginMethod
    public void listEnvironments(PluginCall call) {
        forward(call, Bridge::listEnvironments);
    }

    // --- History -----------------------------------------------------------

    @PluginMethod
    public void addHistory(PluginCall call) {
        String entryJSON = requireArg(call, "entryJSON");
        if (entryJSON == null) return;
        forward(call, b -> b.addHistory(entryJSON));
    }

    @PluginMethod
    public void listHistory(PluginCall call) {
        forward(call, Bridge::listHistory);
    }

    @PluginMethod
    public void clearHistory(PluginCall call) {
        forward(call, Bridge::clearHistory);
    }

    @PluginMethod
    public void migrateLegacyHistory(PluginCall call) {
        String entriesJSON = requireArg(call, "entriesJSON");
        if (entriesJSON == null) return;
        forward(call, b -> b.migrateLegacyHistory(entriesJSON));
    }

    // --- Settings ----------------------------------------------------------

    @PluginMethod
    public void getSettings(PluginCall call) {
        forward(call, Bridge::getSettings);
    }

    @PluginMethod
    public void saveSettings(PluginCall call) {
        String settingsJSON = requireArg(call, "settingsJSON");
        if (settingsJSON == null) return;
        forward(call, b -> b.saveSettings(settingsJSON));
    }

    // --- Import / Export ---------------------------------------------------

    @PluginMethod
    public void exportCollection(PluginCall call) {
        String id = requireArg(call, "id");
        if (id == null) return;
        forward(call, b -> b.exportCollection(id));
    }

    @PluginMethod
    public void exportEnvironment(PluginCall call) {
        String id = requireArg(call, "id");
        if (id == null) return;
        forward(call, b -> b.exportEnvironment(id));
    }

    @PluginMethod
    public void importData(PluginCall call) {
        String dataJSON = requireArg(call, "dataJSON");
        if (dataJSON == null) return;
        // gomobile escapes the Go method `Import` (a Java keyword-adjacent name)
        // to `import_` on the generated Bridge.
        forward(call, b -> b.import_(dataJSON));
    }

    // --- Metadata ----------------------------------------------------------

    @PluginMethod
    public void appVersion(PluginCall call) {
        // The version is the APK's versionName, stamped by
        // scripts/set-android-version.mjs before the release build (Req 15.5).
        // Read it from PackageManager rather than BuildConfig, because AGP 8
        // disables BuildConfig generation by default.
        String version = "";
        try {
            PackageManager pm = getContext().getPackageManager();
            version = pm.getPackageInfo(getContext().getPackageName(), 0).versionName;
            if (version == null) {
                version = "";
            }
        } catch (Exception e) {
            version = "";
        }
        resolveResult(call, version);
    }
}
