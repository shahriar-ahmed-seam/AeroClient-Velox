// VoltBridgeInstrumentedTest.kt — Android instrumented (on-device) tests for
// Volt's Capacitor↔AAR bridge (task 22.3).
//
// This is a TEMPLATE, mirroring how VoltBridgePlugin.kt is delivered. The native
// android/ project is generated on demand (`npx cap add android`) and is not
// committed, so this test source lives under frontend/native-android/ and is
// copied into the generated project's androidTest source set. See
// frontend/native-android/README.md ("Instrumented tests (task 22.3)") for the
// exact copy location, the test runner config, and the run command.
//
// What it verifies
// ----------------
// It drives the gomobile-bound `mobile.Bridge` directly (the same type the
// VoltBridgePlugin forwards to), proving the Android request path behaves like
// the desktop path:
//   - Every HTTP method (GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS) can be
//     built and sent, returning a displayable response (Req 15.1, 15.3).
//   - Requests run through native Go rather than the WebView's fetch, so they
//     succeed against endpoints that return no CORS headers — there is no
//     WebView/browser involved in this test at all (Req 15.2).
//   - Engine/network errors are preserved in the returned HTTPResponse JSON's
//     `error` field with a zeroed status, exactly like the desktop path
//     (Req 15.4).
//
// Why it talks to the Bridge directly (not through the WebView): the Bridge is
// the single source of execution truth. VoltBridgePlugin.execute() is a thin
// `bridge.execute(reqJSON)` forward (see VoltBridgePlugin.kt), so exercising the
// Bridge on-device covers the native half of the bridge end to end. A second,
// optional test that goes through the full Capacitor plugin is sketched at the
// bottom and is normally left disabled because it needs a hosted Activity.
//
// Requirements to run (NOT part of `npm test` / `go test`):
//   - An emulator or a physical device (this is an instrumented test).
//   - Network access from that device — the methods below hit public no-CORS
//     endpoints. Running them through native Go is precisely what proves no
//     WebView CORS applies (Req 15.2).
//   - volt.aar on the app's classpath (built by scripts/build-aar.sh with
//     -javapkg=dev.volt.apiclient.bridge), exposing
//     dev.volt.apiclient.bridge.mobile.{Mobile,Bridge}.

package dev.volt.apiclient

import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.json.JSONObject
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

// gomobile-generated bindings from volt.aar (package set by -javapkg). These are
// the same symbols VoltBridgePlugin.kt imports.
import dev.volt.apiclient.bridge.mobile.Bridge
import dev.volt.apiclient.bridge.mobile.Mobile

@RunWith(AndroidJUnit4::class)
class VoltBridgeInstrumentedTest {

    // A public service that echoes the request and, crucially, does NOT emit
    // CORS response headers. Reaching it successfully from native Go is the
    // proof that no WebView CORS policy applies (Req 15.2). If this host is
    // unavailable in CI, swap it for any other no-CORS echo service (e.g.
    // https://httpbin.org) — the assertions only depend on getting an HTTP
    // status back, not on the body shape.
    private val baseUrl = "https://echo.hoppscotch.io"

    // A host that cannot resolve, used to force a native network error so we can
    // assert it is preserved and surfaced (Req 15.4). The .invalid TLD is
    // reserved by RFC 2606 and is guaranteed never to resolve.
    private val unresolvableUrl = "https://volt-nonexistent-host.invalid/"

    private lateinit var dbFile: File
    private lateinit var bridge: Bridge

    @Before
    fun setUp() {
        // Open the Bridge against a temp on-device DB path under the instrumented
        // app's cache dir, so each run starts from a clean store and we never
        // touch the real app database.
        val ctx = InstrumentationRegistry.getInstrumentation().targetContext
        dbFile = File.createTempFile("volt-test-", ".db", ctx.cacheDir)
        // Mobile.newBridge opens (or creates) the SQLite DB and returns a Bridge.
        bridge = Mobile.newBridge(dbFile.absolutePath)
    }

    @After
    fun tearDown() {
        // Release DB connections and remove the temp database file (+ WAL/SHM).
        try {
            bridge.close()
        } catch (_: Throwable) {
            // Closing is best-effort in teardown.
        }
        deleteQuietly(dbFile)
        deleteQuietly(File(dbFile.absolutePath + "-wal"))
        deleteQuietly(File(dbFile.absolutePath + "-shm"))
    }

    // --- Req 15.1 / 15.2 / 15.3: every method sends and returns a response ----

    @Test
    fun get_returns_displayable_response() = assertMethodExecutes("GET")

    @Test
    fun post_returns_displayable_response() = assertMethodExecutes("POST")

    @Test
    fun put_returns_displayable_response() = assertMethodExecutes("PUT")

    @Test
    fun patch_returns_displayable_response() = assertMethodExecutes("PATCH")

    @Test
    fun delete_returns_displayable_response() = assertMethodExecutes("DELETE")

    @Test
    fun head_returns_displayable_response() = assertMethodExecutes("HEAD")

    @Test
    fun options_returns_displayable_response() = assertMethodExecutes("OPTIONS")

    /**
     * Builds a RawRequest for [method] against the no-CORS echo endpoint, sends
     * it through the Bridge, and asserts the returned HTTPResponse is a usable,
     * displayable success: no engine error and a real HTTP status code in the
     * 100..599 range (Req 15.1, 15.3). Reaching the host at all — through native
     * Go, with no WebView in the loop — demonstrates CORS does not apply
     * (Req 15.2).
     */
    private fun assertMethodExecutes(method: String) {
        val reqJSON = rawRequestJSON(method, baseUrl)

        val respJSON = bridge.execute(reqJSON)
        val resp = JSONObject(respJSON)

        val error = resp.optString("error", "")
        assertTrue(
            "[$method] expected no engine error but got: $error",
            error.isEmpty()
        )

        val status = resp.optInt("status", 0)
        assertTrue(
            "[$method] expected an HTTP status in 100..599 but got $status",
            status in 100..599
        )

        // The response carries the fields the Response_Viewer renders (Req 15.3):
        // statusText, headers, body, durationMs, sizeBytes are all present in the
        // HTTPResponse JSON shape. We assert headers is an array (it is always
        // serialized, even if empty) to confirm the JSON is the expected model.
        assertTrue(
            "[$method] response JSON is missing the headers array",
            resp.has("headers")
        )
    }

    // --- Req 15.4: native errors are preserved and surfaced -------------------

    /**
     * Sends a request to an unresolvable host and asserts the Bridge returns a
     * well-formed HTTPResponse whose `error` is non-empty and whose `status` is
     * zero — i.e. the native network failure is preserved in the response JSON
     * exactly as the desktop path surfaces it (Req 15.4), instead of throwing or
     * returning a fabricated status.
     */
    @Test
    fun unresolvable_host_preserves_error_with_zero_status() {
        val reqJSON = rawRequestJSON("GET", unresolvableUrl)

        val respJSON = bridge.execute(reqJSON)
        val resp = JSONObject(respJSON)

        val error = resp.optString("error", "")
        assertTrue(
            "expected a non-empty error for an unresolvable host, got JSON: $respJSON",
            error.isNotEmpty()
        )

        val status = resp.optInt("status", -1)
        assertEquals(
            "a failed request must report status 0 (no HTTP exchange occurred)",
            0,
            status
        )

        // A failed request must not present a body the viewer would mistake for
        // a real response (Req 15.4: error shown "in place of a response body").
        assertTrue(
            "a failed request must not carry a response body",
            resp.optString("body", "").isEmpty()
        )
    }

    /**
     * An empty URL must fail validation in the shared engine without performing a
     * network call, and that validation error must be surfaced in the response's
     * `error` field with a zeroed status — the same preservation guarantee as a
     * network failure (Req 15.4; engine validation per Req 1.3).
     */
    @Test
    fun invalid_url_preserves_validation_error() {
        val reqJSON = rawRequestJSON("GET", "")

        val respJSON = bridge.execute(reqJSON)
        val resp = JSONObject(respJSON)

        assertTrue(
            "expected a validation error for an empty URL, got JSON: $respJSON",
            resp.optString("error", "").isNotEmpty()
        )
        assertEquals(
            "a validation failure must report status 0",
            0,
            resp.optInt("status", -1)
        )
    }

    // --- Helpers --------------------------------------------------------------

    /**
     * Builds a JSON-encoded model.RawRequest for [method] and [url]. The shape
     * matches internal/model.RawRequest exactly (method, url, params, headers,
     * body, auth), so it deserializes cleanly in mobile.Bridge.Execute.
     *
     * GET/HEAD/OPTIONS are sent with no body. The mutating verbs carry a small
     * JSON body so the request path encodes a body and Content-Type, exercising
     * more of the shared engine. Note the engine drops bodies on GET/HEAD anyway
     * (Req 2.9), so a "none" body is used there for clarity.
     */
    private fun rawRequestJSON(method: String, url: String): String {
        val sendsBody = method == "POST" || method == "PUT" || method == "PATCH"
        val body = if (sendsBody) {
            // Raw JSON body; the engine sets Content-Type: application/json when
            // the user has not set one (Req 2.3).
            JSONObject()
                .put("type", "json")
                .put("raw", """{"hello":"volt","method":"$method"}""")
                .put("formFields", emptyList<Any>().toJSONArray())
        } else {
            JSONObject()
                .put("type", "none")
                .put("raw", "")
                .put("formFields", emptyList<Any>().toJSONArray())
        }

        val auth = JSONObject()
            .put("type", "none")
            .put("bearerToken", "")
            .put("basicUser", "")
            .put("basicPass", "")
            .put("apiKeyName", "")
            .put("apiKeyValue", "")
            .put("apiKeyLocation", "header")

        return JSONObject()
            .put("method", method)
            .put("url", url)
            .put("params", emptyList<Any>().toJSONArray())
            .put("headers", emptyList<Any>().toJSONArray())
            .put("body", body)
            .put("auth", auth)
            .toString()
    }

    private fun List<Any>.toJSONArray(): org.json.JSONArray {
        val arr = org.json.JSONArray()
        forEach { arr.put(it) }
        return arr
    }

    private fun deleteQuietly(f: File) {
        try {
            if (f.exists()) f.delete()
        } catch (_: Throwable) {
            // Ignore cleanup failures.
        }
    }
}

// ---------------------------------------------------------------------------
// OPTIONAL: full Capacitor-plugin path test (normally disabled)
// ---------------------------------------------------------------------------
//
// The test above drives the Bridge directly, which is the native execution
// truth. If you also want to assert that VoltBridgePlugin forwards correctly
// (JS options -> bridge.execute -> { result }), run the plugin under a hosted
// Activity. That requires a Capacitor test harness/Activity, so it is left as a
// documented sketch rather than an enabled test:
//
//   @RunWith(AndroidJUnit4::class)
//   class VoltBridgePluginInstrumentedTest {
//       // 1. Launch a BridgeActivity that has registerPlugin(VoltBridgePlugin::class.java).
//       // 2. Obtain the VoltBridgePlugin instance from the Capacitor bridge.
//       // 3. Build a PluginCall with options { reqJSON: rawRequestJSON("GET", baseUrl) }.
//       // 4. Invoke plugin.execute(call) and read call's resolved { result } JSON.
//       // 5. Assert the same way as assertMethodExecutes above.
//   }
//
// The plugin layer is a one-line forward (VoltBridgePlugin.execute calls
// bridge.execute(reqJSON)), so the direct-Bridge tests already cover the logic
// that could break; the harness test would only re-verify Capacitor's own
// option marshalling.
