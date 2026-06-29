# Volt Android (Capacitor) Shell

This document describes how the Volt Android app is produced. It covers task 22.1
(the Capacitor shell that hosts the existing Svelte build) and points to the
follow-up tasks that complete the Android story.

> Design reference: see `.kiro/specs/volt-api-client/design.md`,
> section "Resolving the Android Open Technical Decision". The Android app reuses
> the **exact** Svelte/Vite frontend inside a Capacitor WebView shell and calls
> the shared Go `httpcore`/`store` packages compiled to an Android library
> (`.aar` via `gomobile bind`) through a custom Capacitor plugin.

## What task 22.1 sets up

- `@capacitor/core`, `@capacitor/cli`, and `@capacitor/android` are declared in
  `package.json`.
- `capacitor.config.ts` configures the shell:
  - `appId`: `dev.volt.apiclient`
  - `appName`: `Volt`
  - `webDir`: `dist` (the Vite build output — the same bundle the desktop build uses)
  - `server.androidScheme`: `https` (secure origin for the bundled WebView)
- npm scripts for the mobile flow:
  - `npm run cap:add:android` → `cap add android` (generates the native project)
  - `npm run cap:sync` → `cap sync` (copies `dist/` + plugins into the native project)
  - `npm run android:build` → `vite build && cap sync android`
  - `npm run android:set-version` → stamps the Semantic_Version into the native project

## Shared design system and responsive layout (Req 15.6, 15.9)

Because Capacitor hosts the **same** `dist/` produced by `vite build`, the Android
app automatically inherits the shared design system (the `tokens.css` color and
typography tokens, the global `border-radius: 0` rule, the single spacing scale)
and the narrow-viewport responsive layout that already exist in the Svelte app.
There is **no separate Android UI** and no UI duplication — the narrow breakpoints
defined for Requirement 12 are exactly what render on a phone-sized WebView.

## Version injection (Req 15.5, 15.9)

The Android `Semantic_Version` lives in the Capacitor-generated
`android/app/build.gradle` (`versionName` / `versionCode`).

`scripts/set-android-version.mjs` is the mechanism that stamps it:

```bash
# explicit version
node scripts/set-android-version.mjs 1.4.2
# or via npm, falling back to VOLT_VERSION env then package.json "version"
VOLT_VERSION=1.4.2 npm run android:set-version
```

The script:
- accepts a version from argv, then `VOLT_VERSION`, then `package.json`'s `version`;
- validates it is `MAJOR.MINOR.PATCH`;
- derives a monotonic `versionCode` as `MAJOR*10000 + MINOR*100 + PATCH`;
- rewrites `versionName`/`versionCode` in `android/app/build.gradle`;
- is a **no-op (exit 0) when the native project is not present**, so it never
  breaks a build in an environment without the Android project.

The release workflow (task 22.4) calls this with the pushed git tag before
building the APK/AAB. The in-app About/Settings view reads the running version
through the Capacitor backend's `appVersion()` once the native bridge lands
(task 22.2).

> Note: the desktop/web Vite build is intentionally left untouched — no Vite
> `define` is required for version injection because the Android version flows
> through `build.gradle`, and the desktop version flows through the Wails
> `AppVersion()` binding.

## Generating the native Android project (manual / CI step)

The native `android/` project is **not committed**. It is generated on demand and
requires a provisioned environment (JDK + Android SDK):

```bash
cd frontend
npm install                 # installs Capacitor (needs network access)
npm run build               # produces dist/ (the web bundle Capacitor wraps)
npx cap add android         # generates the android/ native project (needs Android SDK)
npm run cap:sync            # copies dist/ + plugins into android/
npm run android:set-version 1.4.2   # stamps versionName/versionCode
```

These steps are not run as part of `npm run build` or `npm test` and have no
effect on the desktop/web build. If `npm install` cannot reach the network in
this environment, the Capacitor dependencies are still declared in
`package.json` so the install + `npx cap add android` can be completed later in
CI or a dev machine that has network and the Android SDK.

## What is NOT part of task 22.1

- The `gomobile` AAR build and the custom Capacitor bridge plugin, plus the real
  `capacitorBackend` wiring that routes requests through native Go (bypassing
  WebView CORS) and persists on-device — **task 22.2** (now implemented, see
  below).
- The release workflow that builds and publishes the Android artifact on semver
  tags — **task 22.4**.

## Task 22.2 — gomobile AAR + Capacitor bridge plugin

Task 22.2 wires the Android app to the shared Go core so requests run through
native Go (no WebView CORS, Req 15.2), reuse the desktop `httpcore` engine
(Req 15.1), preserve engine/network errors (Req 15.4), and persist on-device
(Req 15.5). The pieces:

- **AAR build script:** `scripts/build-aar.sh` (module root) runs
  `gomobile bind -target=android -androidapi <api> -o frontend/android/app/libs/volt.aar ./mobile`
  against the facade in `mobile/bind.go`. It documents the required toolchain
  (`go install golang.org/x/mobile/cmd/gomobile@latest`, `gomobile init`) and
  env (`ANDROID_HOME`/`ANDROID_NDK_HOME`).
- **JS plugin definition:** `src/lib/native/voltBridgePlugin.ts` declares the
  `VoltBridge` plugin interface mirroring `mobile.Bridge` (string-in/string-out)
  and registers it with `registerPlugin` (safe to import on web/desktop — it
  loads no native code).
- **Native plugin template:** `native-android/VoltBridgePlugin.kt` plus
  `native-android/README.md` with the copy/registration steps into the generated
  `android/` project.
- **`capacitorBackend`:** `src/lib/backend/capacitor.ts` now calls the plugin,
  marshals args to JSON, parses the returned JSON string (throwing on the
  `{"error":...}` envelope, treating `{"ok":true}` as void), and returns typed
  results. `getBackend()` selects it only when `window.Capacitor` is present, so
  the desktop/web build is unaffected.

The AAR build and native project steps require the Android SDK/NDK + gomobile
toolchain and are run in CI or on a provisioned dev machine; they are **not**
part of `npm run build`/`check`/`test`. See `native-android/README.md`.

