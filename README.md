<div align="center">

# ⚡ AeroClient-Velox

**A fast, cross-platform API client for desktop and Android.**

Build, send, and inspect HTTP requests; organize them into collections; manage
environments and variables; and review request history — all backed by a single
shared Go core that runs natively on every platform.

</div>

---

## Overview

Volt is an API client (in the spirit of Postman/Insomnia/Hoppscotch) built around
one principle: **the request engine and persistence layer are written once in Go
and reused everywhere.** The desktop app embeds that core via [Wails](https://wails.io);
the Android app embeds the same core compiled to an `.aar` via
[`gomobile`](https://pkg.go.dev/golang.org/x/mobile) and hosted in a
[Capacitor](https://capacitorjs.com) shell. The user interface — a
[Svelte 5](https://svelte.dev) single-page app — is identical on both platforms.

Because requests execute in native Go rather than the WebView, Volt is **not
subject to browser CORS** and behaves consistently across platforms.

## Features

- **Request builder** — all HTTP methods, query params with bidirectional URL sync,
  headers, body (JSON / text / form-data / x-www-form-urlencoded), and auth
  (Bearer / Basic / API key in header or query).
- **Response viewer** — status/time/size, pretty/raw/preview modes, syntax-highlighted
  JSON, sandboxed HTML preview, header inspection, copy-to-clipboard, and 5 MB
  truncation handling.
- **Collections & folders** — nestable up to 10 levels, drag-free move, persisted in SQLite.
- **Environments & variables** — `{{variable}}` interpolation with inline unresolved-token
  hints; exactly one active environment at a time.
- **History** — automatic, capped at 1000 entries, restore any past request's full config.
- **Import / export** — versioned JSON envelopes with collision-safe naming and atomic writes.
- **Command palette & shortcuts** — Ctrl/Cmd+K palette, Ctrl/Cmd+Enter send, Ctrl/Cmd+S save.
- **Design system** — token-driven, sharp-cornered UI on a 4px spacing scale, fully responsive
  from 320px to 3840px (multi-column → collapsible sidebar → bottom sheets).
- **Settings** — light/dark/system theme, TLS-verification toggle, request timeout (1–600s).

## Architecture

```
                 ┌──────────────────────────────────────────────┐
                 │            Svelte 5 frontend (UI)             │
                 │   components · runes stores · Backend iface   │
                 └───────────────┬───────────────┬──────────────┘
                                 │               │
                   wailsBackend  │               │  capacitorBackend
                                 ▼               ▼
                 ┌───────────────────┐   ┌────────────────────────┐
                 │  Wails bindings   │   │  Capacitor VoltBridge  │
                 │     (app.go)      │   │  plugin → volt.aar     │
                 └─────────┬─────────┘   └───────────┬────────────┘
                           │                         │
                           ▼                         ▼
                 ┌──────────────────────────────────────────────┐
                 │              Shared Go core                   │
                 │  internal/httpcore   ·   internal/store       │
                 │  (prepare + execute) ·   (SQLite persistence) │
                 │              internal/model                   │
                 └──────────────────────────────────────────────┘
```

The same `internal/httpcore` and `internal/store` packages power both platforms,
so request preparation, execution, and persistence are guaranteed identical.

## Tech stack

| Layer        | Technology                                              |
| ------------ | ------------------------------------------------------- |
| Core / engine| Go (`net/http`), `modernc.org/sqlite` (pure-Go, no cgo) |
| Desktop      | Wails v2                                                |
| Android      | gomobile (`.aar`) + Capacitor                           |
| Frontend     | Svelte 5 (runes), TypeScript, Vite                      |
| Tests        | Go `testing/quick` (property-based), Vitest + fast-check |
| CI/CD        | GitHub Actions                                          |

## Project structure

```
volt/
├── app.go                  # Wails bindings (desktop) over the shared core
├── main.go                 # Wails entry point; `version` injected via -ldflags
├── internal/
│   ├── model/              # Shared data types (JSON-tagged, mirrored in TS)
│   ├── httpcore/           # Pure request preparation + execution engine
│   └── store/              # SQLite persistence (collections, env, history, settings)
├── mobile/
│   └── bind.go             # gomobile-bindable facade (string-in/JSON-out)
├── internal/release/       # Semver release-tag gating helper
├── frontend/
│   ├── src/
│   │   ├── lib/            # models, Backend interface (wails/capacitor), runes stores
│   │   ├── components/     # request / response / sidebar / command / settings UI
│   │   └── styles/        # design-system tokens
│   ├── native-android/     # Capacitor VoltBridge plugin (Kotlin) + instrumented tests
│   ├── capacitor.config.ts
│   └── scripts/            # Android version stamping
├── scripts/build-aar.sh    # gomobile bind → volt.aar
└── .github/workflows/      # ci.yml (build+test) · release.yml (tagged releases)
```

## Prerequisites

- **Go** (see `go.mod` for the version) and **Node 20+**.
- **Desktop:** the [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation) and
  a platform WebView (WebView2 on Windows; WebKit on macOS/Linux).
- **Android (optional):** JDK 17, the Android SDK + NDK, and the gomobile toolchain
  (`go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init`).

## Development

```bash
# Live desktop dev with hot-reload frontend
wails dev

# Frontend only (browser), against the bound Go methods at http://localhost:34115
cd frontend && npm install && npm run dev
```

## Building

```bash
# Desktop (current OS). Inject the version so the app reports it.
wails build -ldflags "-X main.version=v1.0.0"
# → build/bin/volt(.exe)

# Android (requires Android SDK/NDK + gomobile)
scripts/build-aar.sh --api 24          # builds frontend/android/app/libs/volt.aar
cd frontend && npm run build && npx cap add android && npm run cap:sync
npm run android:set-version 1.0.0 && (cd android && ./gradlew assembleRelease)
```

See [`frontend/native-android/README.md`](frontend/native-android/README.md) for the
full Android bridge wiring.

## Testing

```bash
# Go core/store/bindings (includes property-based tests)
go test ./...

# Frontend (Vitest + fast-check; jsdom for component tests)
cd frontend && npm test

# Type-check the frontend (treated as a CI gate)
cd frontend && npm run check
```

Property-based tests are tagged `Feature: volt-api-client, Property N: …` and live
alongside the logic they validate.

## Releasing

Releases are produced by GitHub Actions, triggered **only** by a semantic-version tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

`.github/workflows/release.yml` then:

- builds desktop artifacts for **Windows, macOS, and Linux** (`wails build`, version
  injected from the tag) and publishes them to a single GitHub Release;
- builds the **Android** APK independently — its publish is gated separately, so a
  failed Android build never blocks the desktop release and vice versa.

Every push and pull request runs `.github/workflows/ci.yml` (Go build+test, frontend
build, `svelte-check`, and the Vitest suite).

## License

See [LICENSE](LICENSE) if present; otherwise all rights reserved by the project owner.
