# Volt Android native bridge (tasks 22.2, 22.3)

This directory holds the **native Android pieces** of Volt's custom Capacitor
plugin that are copied into the generated `android/` project. The native project
is produced on demand by `npx cap add android` and is not committed, so the
plugin source lives here as a template plus these instructions.

> Design reference: `.kiro/specs/volt-api-client/design.md`,
> "Resolving the Android Open Technical Decision" and the Bindings section.

## How the bridge fits together

```
Svelte UI  ──▶  Backend (capacitorBackend)  ──▶  VoltBridge plugin (TS)
                src/lib/backend/capacitor.ts      src/lib/native/voltBridgePlugin.ts
                                                          │  Capacitor JS↔native bridge
                                                          ▼
                                            VoltBridgePlugin.kt  (this dir)
                                                          │  JNI (gomobile)
                                                          ▼
                                            volt.aar  →  mobile.Bridge  →  httpcore + store
```

- Requests run through **native Go**, not the WebView's `fetch`, so they are not
  subject to browser **CORS** (Req 15.2).
- The **same `httpcore` engine** the desktop build uses prepares and executes
  requests, guaranteeing parity (Req 15.1) and preserving engine/network errors
  in the returned `HTTPResponse.error` (Req 15.4).
- The Bridge's **on-device SQLite** store persists Collections, Environments,
  and History across restarts (Req 15.5).

## Files

- `VoltBridgePlugin.kt` — the `@CapacitorPlugin(name = "VoltBridge")` class. Each
  method forwards a string-in/string-out call to the matching `mobile.Bridge`
  method and resolves `{ result: <json string> }`.
- `androidTest/VoltBridgeInstrumentedTest.kt` — on-device instrumented test
  (task 22.3) that drives `mobile.Bridge` directly to prove the Android request
  path matches the desktop path. See "Instrumented tests (task 22.3)" below.

## Build + wiring steps (manual / CI, requires Android SDK + NDK)

These steps are **not** part of `npm run build`/`check`/`test`. They require a
provisioned environment (JDK, Android SDK + NDK, Go, gomobile).

1. **Build the AAR** from the Go facade (`mobile/bind.go`):

   ```bash
   # from the module root (volt/)
   go install golang.org/x/mobile/cmd/gomobile@latest
   go install golang.org/x/mobile/cmd/gobind@latest
   export PATH="$PATH:$(go env GOPATH)/bin"
   export ANDROID_HOME=/path/to/Android/sdk
   export ANDROID_NDK_HOME=$ANDROID_HOME/ndk/<version>
   gomobile init
   scripts/build-aar.sh --api 24
   ```

   This writes `frontend/android/app/libs/volt.aar`. The build uses
   `-javapkg=dev.volt.apiclient.bridge`, so the generated Java package is
   `dev.volt.apiclient.bridge.mobile` with:
   - `dev.volt.apiclient.bridge.mobile.Mobile.newBridge(path)` — the constructor, and
   - `dev.volt.apiclient.bridge.mobile.Bridge` — the bound type.

2. **Generate the native project** (if not already done) and sync the web build:

   ```bash
   cd frontend
   npm run build
   npx cap add android      # generates android/
   npm run cap:sync
   ```

3. **Make Gradle consume the AAR.** In `android/app/build.gradle` add (once):

   ```gradle
   dependencies {
       implementation fileTree(dir: 'libs', include: ['*.aar'])
       // ...existing Capacitor deps
   }
   ```

   (The AAR is already placed at `android/app/libs/volt.aar` by step 1.)

4. **Copy the plugin into the project** and register it. Copy
   `VoltBridgePlugin.kt` into the app's package source dir, e.g.
   `android/app/src/main/java/dev/volt/apiclient/VoltBridgePlugin.kt`, then
   register it in `MainActivity`:

   ```kotlin
   import dev.volt.apiclient.VoltBridgePlugin

   class MainActivity : BridgeActivity() {
       override fun onCreate(savedInstanceState: Bundle?) {
           registerPlugin(VoltBridgePlugin::class.java)
           super.onCreate(savedInstanceState)
       }
   }
   ```

5. **Stamp the version** and build:

   ```bash
   npm run android:set-version 1.4.2   # writes versionName/versionCode
   cd android && ./gradlew assembleRelease
   ```

## Instrumented tests (task 22.3)

`androidTest/VoltBridgeInstrumentedTest.kt` is an on-device (instrumented) test
that exercises the bridge end to end against real, public, **no-CORS** endpoints.
It drives the gomobile-bound `mobile.Bridge` directly — the same type
`VoltBridgePlugin.execute()` forwards to — to verify the Android request path
behaves like the desktop path:

- Each HTTP method (GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS) builds a
  `model.RawRequest` JSON, calls `bridge.execute(reqJSON)`, and asserts the
  returned `HTTPResponse` JSON has no `error` and a real status in `100..599`
  (Req 15.1, 15.3).
- Because the request runs through **native Go** (no WebView/`fetch` involved),
  reaching a host that returns no CORS headers proves browser CORS does not apply
  (Req 15.2).
- A request to an unresolvable host (`*.invalid`) asserts the returned JSON has a
  **non-empty `error` and `status: 0`**, proving native errors are preserved and
  surfaced like the desktop path (Req 15.4). An empty-URL case covers engine
  validation errors the same way.

### Where to place it

Copy the file into the generated project's **androidTest** source set, under the
app package (matching its `package dev.volt.apiclient;`):

```
android/app/src/androidTest/java/dev/volt/apiclient/VoltBridgeInstrumentedTest.kt
```

It depends on `volt.aar` being on the classpath (the same dependency the main
plugin uses — see step 3 above), so no extra wiring is needed beyond the
`fileTree(... '*.aar')` dependency.

### Test runner config

The instrumented tests use the AndroidX JUnit4 runner. In `android/app/build.gradle`
ensure (Capacitor projects already include these AndroidX test deps and runner):

```gradle
android {
    defaultConfig {
        testInstrumentationRunner "androidx.test.runner.AndroidJUnitRunner"
    }
}

dependencies {
    androidTestImplementation "androidx.test.ext:junit:1.1.5"
    androidTestImplementation "androidx.test:runner:1.5.2"
    // org.json is part of the Android platform — no dependency needed.
}
```

### Running it

Requires a running **emulator or connected device** *and* **network access** on
that device:

```bash
cd android
./gradlew connectedAndroidTest
```

> This is a **manual / CI-with-emulator** step. It is intentionally **not** part
> of `npm test`, `npm run check`, or `go test`, because it needs a device and
> live network. Run it under an emulator locally or in a CI job that provisions
> one (e.g. an AVD or a device-farm runner).

## Notes / gotchas

- **Reserved method name.** Go's `Bridge.Import` binds to a Java/Kotlin method
  whose name collides with the `import` keyword; gomobile escapes it (commonly
  `import_`). `VoltBridgePlugin.kt` calls `bridge.import_(dataJSON)`. If your
  gomobile version generates a different escaped name, adjust that one call to
  match the symbol in the generated `Bridge` class.
- **API level.** `scripts/build-aar.sh --api <n>` must be `>=` the project's
  `minSdkVersion`.
- **Plugin name must match.** The JS side registers `registerPlugin('VoltBridge')`
  and the Kotlin side is `@CapacitorPlugin(name = "VoltBridge")`. Keep both in
  sync or the bridge calls will not resolve.
- **`appVersion()`** has no Bridge counterpart: it returns `BuildConfig.VERSION_NAME`
  so the in-app About/Settings view shows the version baked into the APK.
- **Instrumented test network.** `VoltBridgeInstrumentedTest` hits a live public
  echo host (`https://echo.hoppscotch.io`); if that host is unavailable in CI,
  swap `baseUrl` for another no-CORS service (e.g. `https://httpbin.org`). The
  assertions only require that an HTTP status comes back, not a specific body.
  The error-preservation test relies on the reserved `*.invalid` TLD never
  resolving, so it needs no live host.
