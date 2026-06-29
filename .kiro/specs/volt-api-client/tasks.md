# Implementation Plan: Volt API Client

## Overview

This plan evolves Volt from a single-screen Wails prototype into a cross-platform API client. The work proceeds bottom-up: first the shared, pure Go core (`internal/model`, `internal/httpcore`), then durable persistence (`internal/store`), then the platform bindings (Wails + gomobile), then the Svelte frontend (design system, stores, components, responsive layout), and finally the CI/CD workflows and Android packaging. Each step builds on the previous one and ends by wiring its output into the layer above it, so no code is left orphaned.

Property-based tests are placed immediately next to the pure logic they validate, annotated with their design property number and the requirement clauses they check. Test sub-tasks are marked optional with `*`.

## Tasks

- [x] 1. Set up shared model package and Go module layout
  - Create `internal/model` package with the shared structs: `Request`/`RawRequest`, `KeyValue`, `BodySpec`, `AuthSpec`, `SavedRequest`, `Folder`, `Collection`, `Variable`, `Environment`, `HistoryEntry`, `Settings`, `HTTPResponse` (with `Truncated`), and the `Method`/`BodyType`/`AuthType`/`ApiKeyLocation` type aliases, all with JSON tags
  - Add `golang.org/x/mobile` and `modernc.org/sqlite` to `go.mod`; confirm the module builds with `go build ./...`
  - _Requirements: 5.1, 7.1, 9.1_

  - [x]* 1.1 Write unit tests for model JSON marshal/unmarshal
    - Verify each struct round-trips through `encoding/json` with expected field names
    - _Requirements: 5.1, 7.1_

- [x] 2. Implement variable interpolation in httpcore
  - [x] 2.1 Implement `InterpolateString(in, env)` in `internal/httpcore`
    - Replace `{{name}}` tokens with case-sensitive Variable matches from the active environment; collect unresolved tokens and send them through literally
    - _Requirements: 6.4, 6.5, 6.8_

  - [x]* 2.2 Write property test for interpolation
    - **Property 8: Variable interpolation resolves defined tokens and passes unresolved tokens through literally**
    - **Validates: Requirements 6.4, 6.5, 6.6, 6.8**

- [x] 3. Implement URL validation and parameter/header assembly in httpcore
  - [x] 3.1 Implement URL validation and query-param merge
    - Reject empty URL, missing scheme, non-http(s) scheme, and missing host with a `ValidationError` naming the failed rule and performing no network call; percent-encode enabled non-empty params into the query; exclude disabled/empty rows
    - Set enabled non-empty headers, preserving duplicate header names via `Header.Add`
    - _Requirements: 1.3, 1.4, 1.5, 1.6_

  - [x]* 3.2 Write property test for invalid-URL rejection
    - **Property 3: Invalid URLs are rejected without a network call**
    - **Validates: Requirements 1.3**

  - [x]* 3.3 Write property test for request assembly
    - **Property 1: Request assembly includes enabled rows, excludes disabled rows, and preserves method and duplicate header names**
    - **Validates: Requirements 1.2, 1.4, 1.5, 1.6**

- [x] 4. Implement request body encoding in httpcore
  - [x] 4.1 Implement `EncodeBody(b, method)`
    - None → no body, no Content-Type; Raw JSON → `application/json` default; Plain Text → `text/plain` default; honor a user-set Content-Type override; encode `urlencoded` and `form-data` (with generated boundary) excluding disabled/empty-key rows; force empty body for GET/HEAD
    - _Requirements: 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 2.9_

  - [x]* 4.2 Write property test for body encoding and Content-Type
    - **Property 4: Body encoding and Content-Type rules**
    - **Validates: Requirements 2.2, 2.3, 2.4, 2.5, 2.9**

  - [x]* 4.3 Write property test for form body round-trip
    - **Property 5: Form body round-trip excludes disabled and empty-key rows**
    - **Validates: Requirements 2.6, 2.7, 2.8**

- [x] 5. Implement authorization derivation in httpcore
  - [x] 5.1 Implement `DeriveAuthHeader(a)`
    - Bearer/Basic/API-Key (header and query) derivation with whitespace-only guards; Basic base64 of `user:pass` with missing password as empty; ensure a produced `Authorization` header overrides any headers-table `Authorization` row
    - _Requirements: 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 3.9_

  - [x]* 5.2 Write property test for authorization derivation and precedence
    - **Property 7: Authorization derivation across all types with header precedence**
    - **Validates: Requirements 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 3.9**

- [x] 6. Assemble PrepareRequest and implement Execute
  - [x] 6.1 Wire `PrepareRequest(r, env, s)` to combine interpolation, validation, param merge, header handling, body encoding, and auth derivation into a `PreparedRequest`
    - _Requirements: 1.2, 6.4, 6.5_

  - [x] 6.2 Implement `Execute(ctx, doer, pr, s)` with the injectable `Doer`
    - Apply `context.WithTimeout` from `Settings.timeoutSeconds`, apply TLS-skip setting, capture network/timeout errors into `HTTPResponse.Error` with zeroed status/duration/size, and set `Truncated` plus first-5,242,880-byte body when the response exceeds 5 MB
    - _Requirements: 4.9, 4.11, 9.3, 9.5_

  - [x]* 6.3 Write unit tests for Execute error and limit paths
    - Mock `Doer` for network error, timeout (slow Doer), and >5 MB body truncation
    - _Requirements: 4.9, 4.11, 9.5_

  - [x]* 6.4 Write property test for large-body truncation boundary
    - **Property 11: Large response bodies are truncated at the 5 MB boundary**
    - **Validates: Requirements 4.11**

- [x] 7. Checkpoint - httpcore complete
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Implement persistence schema and collections/requests storage
  - [x] 8.1 Create `internal/store` with the SQLite schema and `Store` initialization
    - Use `modernc.org/sqlite` (no cgo); create tables `collections`, `folders`, `requests`, `environments`, `variables`, `history`, `settings`, `meta` (`schema_version`, `legacy_history_migrated`); run all writes in transactions that roll back on failure
    - _Requirements: 5.6, 5.9, 9.7_

  - [x] 8.2 Implement collection/folder/request CRUD and `ListTree`
    - `SaveCollection`, `RenameCollection`, `DeleteCollection`, `SaveRequest`, `ListTree`, preserving names, nesting, and order
    - _Requirements: 5.1, 5.2, 5.5, 5.6_

  - [x]* 8.3 Write property test for persistence round-trip
    - **Property 12: Persistence round-trip across reopen**
    - **Validates: Requirements 5.1, 5.5, 5.6, 6.7, 7.5, 9.7, 15.5**

  - [x] 8.4 Implement `MoveRequest(requestID, targetParentID)`
    - Place the request in the target and remove it from its prior location atomically
    - _Requirements: 5.4_

  - [x]* 8.5 Write property test for move semantics
    - **Property 13: Moving a request leaves it in exactly one location**
    - **Validates: Requirements 5.4**

  - [x] 8.6 Implement folder nesting-depth enforcement (≤10 levels)
    - _Requirements: 5.3_

  - [x]* 8.7 Write property test for folder depth bound
    - **Property 14: Folder nesting depth is bounded at 10**
    - **Validates: Requirements 5.3**

  - [x] 8.8 Implement name validation backstop
    - Collection/Request name 1..255, Environment name 1..64 unique, Variable name 1..128 unique within environment, Variable value 0..4096; reject leaving data unchanged
    - _Requirements: 5.8, 6.1, 6.2, 6.9_

  - [x]* 8.9 Write property test for name validation
    - **Property 15: Name validation by length and uniqueness**
    - **Validates: Requirements 5.8, 6.1, 6.2, 6.9**

- [x] 9. Implement environments, history, and settings storage
  - [x] 9.1 Implement environment CRUD and active-environment management
    - `SaveEnvironment`, `DeleteEnvironment`, `ListEnvironments`, `SetActiveEnvironment` ensuring at most one active and clearing active when the active environment is deleted
    - _Requirements: 6.1, 6.2, 6.3, 6.7, 6.10_

  - [x]* 9.2 Write property test for active-environment invariant
    - **Property 16: Exactly one (or zero) active environment**
    - **Validates: Requirements 6.3, 6.10**

  - [x] 9.3 Implement history `AddHistory`/`ListHistory`/`ClearHistory`
    - Record full entries for success and failure, return reverse-chronological order, enforce the 1000-entry cap discarding oldest
    - _Requirements: 7.1, 7.2, 7.3, 7.5, 7.6_

  - [x]* 9.4 Write property test for history recording and order
    - **Property 17: History records complete entries in reverse chronological order and restores configuration**
    - **Validates: Requirements 7.1, 7.2, 7.3, 7.4**

  - [x]* 9.5 Write property test for history cap
    - **Property 18: History is capped at 1000 entries discarding the oldest**
    - **Validates: Requirements 7.6**

  - [x] 9.6 Implement `MigrateLegacyHistory` guarded by the `legacy_history_migrated` flag
    - Import once and mark complete; on failure preserve legacy data and leave the flag unset for retry
    - _Requirements: 7.8, 7.9_

  - [x]* 9.7 Write property test for migration idempotency
    - **Property 19: Legacy history migration is idempotent**
    - **Validates: Requirements 7.8**

  - [x] 9.8 Implement `GetSettings`/`SaveSettings` with defaults and timeout validation
    - First-launch defaults (System theme, TLS on, 30s); accept timeout only within 1..600 retaining the previous value otherwise
    - _Requirements: 9.6, 9.7, 9.8_

  - [x]* 9.9 Write property test for timeout validation
    - **Property 22: Timeout value validation**
    - **Validates: Requirements 9.6**

- [x] 10. Implement import/export in store
  - [x] 10.1 Implement `ExportCollection`/`ExportEnvironment` and `Import`
    - Emit the documented `voltFormat`/`version` JSON; import within a single transaction recreating entries with new IDs, treating name collisions as separate new entries, atomic temp-file-then-rename on export, full rejection on invalid JSON / malformed structure / unsupported version leaving data unchanged
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6, 8.7_

  - [x]* 10.2 Write property test for import/export round-trip
    - **Property 20: Import/export round-trip with collision-safe naming**
    - **Validates: Requirements 8.1, 8.2, 8.3, 8.4, 8.6**

  - [x]* 10.3 Write property test for atomic rejection of bad imports
    - **Property 21: Malformed or unsupported imports are rejected atomically**
    - **Validates: Requirements 8.5**

- [x] 11. Checkpoint - core and store complete
  - Ensure all tests pass, ask the user if questions arise.

- [x] 12. Implement platform bindings
  - [x] 12.1 Expand Wails `app.go` to bind store CRUD and `ExecuteRequest`
    - Bind `ExecuteRequest` (calls `PrepareRequest` + `Execute`, records history), tree/collection/folder/request/environment/history/settings/import-export methods, and `AppVersion()` returning the embedded `main.version`
    - _Requirements: 1.2, 5.1, 7.1, 13.1_

  - [x] 12.2 Implement gomobile facade `mobile/bind.go`
    - String-in/string-out (JSON) wrappers over the same `httpcore` and `store` calls for `gomobile bind`
    - _Requirements: 15.1, 15.2, 15.5_

  - [x]* 12.3 Write unit tests for binding JSON marshalling
    - Verify the mobile facade round-trips request/response JSON identically to the Wails path
    - _Requirements: 15.1, 15.2_

- [x] 13. Build the frontend design system foundation
  - [x] 13.1 Create the design-system token layer
    - Define color and typography tokens, the single spacing scale (multiples of a 4px base), a global `border-radius: 0` rule, and ≥16px container padding; replace hardcoded values in `style.css`
    - _Requirements: 11.1, 11.2, 11.3, 11.4_

  - [x] 13.2 Implement the `Backend` interface and `wailsBackend`
    - Define the TypeScript `Backend` interface and the Wails-bound implementation; add startup selection between `wailsBackend` and `capacitorBackend`
    - _Requirements: 1.2, 5.1, 13.1_

  - [x] 13.3 Implement Svelte 5 runes stores
    - `requestStore`, `collectionsStore`, `environmentsStore`, `historyStore`, `settingsStore`, `uiStore`, `interpolationStore`, all routing mutations through `Backend`
    - _Requirements: 5.5, 6.6, 7.4, 9.7_

- [x] 14. Implement request-building UI
  - [x] 14.1 Build UrlBar, MethodSelect, and ParamsTable with bidirectional URL↔params sync
    - Method default GET; edit URL updates params table and vice versa
    - _Requirements: 1.1, 1.7, 1.8_

  - [x]* 14.2 Write property test for URL↔param round-trip
    - **Property 2: URL and parameter table round-trip**
    - **Validates: Requirements 1.7, 1.8**

  - [x] 14.3 Implement HeadersTable, AuthPanel, and BodyEditor with a shared JSON Prettify helper
    - Body-type and auth-type selectors with correct defaults; Prettify reformats valid JSON with two-space indent and leaves invalid JSON unchanged with an indication
    - _Requirements: 2.1, 2.10, 2.11, 3.1_

  - [x]* 14.4 Write property test for JSON formatting helper
    - **Property 6: JSON formatting preserves value or leaves invalid input unchanged**
    - **Validates: Requirements 2.10, 2.11, 4.4, 4.5**

  - [x] 14.5 Wire inline unresolved-variable indication via `interpolationStore`
    - _Requirements: 6.6_

- [x] 15. Implement the response viewer
  - [x] 15.1 Build ResponseViewer with StatusBar, BodyView (Pretty/Raw/Preview), and HeadersView
    - Display status/text/elapsed/size, status-class colors, JSON pretty highlighting, HTML preview vs plain text, header order preservation, loading indicator, error rendering, copy-to-clipboard, and the truncation control for >5 MB bodies
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8, 4.9, 4.10, 4.11, 4.12_

  - [x]* 15.2 Write property test for status color classes
    - **Property 9: Response status color matches its status class**
    - **Validates: Requirements 4.2**

  - [x]* 15.3 Write property test for header order preservation
    - **Property 10: Response headers preserve received order**
    - **Validates: Requirements 4.8**

  - [x]* 15.4 Write unit tests for response viewer branches
    - View toggle, HTML preview vs non-previewable, error rendering, loading indicator, copy confirmation
    - _Requirements: 4.3, 4.6, 4.7, 4.9, 4.10, 4.12_

- [x] 16. Implement sidebar: collections, history, and environments
  - [x] 16.1 Build CollectionsTree, HistoryList, and EnvironmentManager
    - Create/rename/delete/move with confirmation prompts for non-empty containers and Clear History; restore request config on history/saved-request selection; environment activation and variable editing with name validation and error preservation
    - _Requirements: 5.2, 5.3, 5.4, 5.7, 6.1, 6.2, 6.3, 6.9, 6.10, 7.3, 7.4, 7.7, 11.7_

  - [x]* 16.2 Write unit tests for confirmation and error branches
    - Delete confirmation accept/decline, clear-history confirmation, failing-store error path preserving input
    - _Requirements: 5.7, 5.9, 7.7, 7.9, 11.7_

- [x] 17. Implement command palette and keyboard shortcuts
  - [x] 17.1 Build CommandPalette with command registry and global keydown handler
    - Ctrl/Cmd+Enter send, Ctrl/Cmd+K palette, Ctrl/Cmd+S save (all `preventDefault`), arrow navigation, Enter run, Escape close, no-results indication
    - _Requirements: 10.1, 10.2, 10.4, 10.5, 10.7, 10.8, 10.9_

  - [x]* 17.2 Write property test for palette filtering
    - **Property 23: Command palette substring filtering**
    - **Validates: Requirements 10.3, 10.7**

  - [x]* 17.3 Write unit tests for shortcut handling
    - `preventDefault` on each shortcut, select/escape/arrow navigation
    - _Requirements: 10.1, 10.2, 10.4, 10.5, 10.8, 10.9_

- [x] 18. Implement settings view and responsive layout
  - [x] 18.1 Build SettingsView
    - Theme (Light/Dark/System) applied live, TLS toggle with persistent warning banner, timeout input rejecting out-of-range values, and the keyboard-shortcut listing
    - _Requirements: 9.1, 9.2, 9.4, 9.6, 10.6_

  - [x] 18.2 Implement responsive layout across breakpoints
    - Wide multi-column (≥1024px), Medium collapsible sidebar (600–1023px), Narrow bottom sheets and stacked/swipeable tabs (<600px), reflow on resize, all controls operable from 320px to 3840px
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6_

  - [x]* 18.3 Write responsive layout tests at representative widths
    - Render at 320/599/600/1023/1024/3840px and assert region/sheet/tab presentation and no off-screen controls
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6_

- [x] 19. Add static design-system audits
  - [x]* 19.1 Write automated design-system audit tests
    - Assert no non-zero `border-radius`, spacing values drawn only from the scale with ≥16px container padding, and no hardcoded color/typography literals outside token files
    - _Requirements: 11.1, 11.2, 11.3, 11.4_

  - [x]* 19.2 Write E2E smoke test for feature reachability
    - Confirm every advertised feature is reachable and operational with no placeholder or permanently disabled controls
    - _Requirements: 11.5, 11.6_

- [x] 20. Checkpoint - desktop app feature-complete
  - Ensure all tests pass, ask the user if questions arise.

- [x] 21. Implement CI and release workflows
  - [x] 21.1 Create `.github/workflows/ci.yml`
    - On push/PR: build Go backend and Svelte frontend, run both test suites, run `svelte-check` treating type errors as failures, report per-step pass/fail status
    - _Requirements: 14.1, 14.2, 14.3, 14.4, 14.5_

  - [x] 21.2 Create `.github/workflows/release.yml` triggered only by semver tags
    - Build desktop artifacts for Windows/macOS/Linux via `wails build`, inject version via `-ldflags`, name artifacts with the version, publish a single GitHub Release with notes, fail the run without publishing on any failed step
    - _Requirements: 13.2, 13.3, 13.4, 13.5_

  - [x]* 21.3 Write property test for semver tag gating
    - **Property 24: Semantic version tag matching gates releases**
    - **Validates: Requirements 13.6**

- [x] 22. Implement Android packaging
  - [x] 22.1 Set up the Capacitor Android shell hosting the Svelte build
    - Configure Capacitor to wrap the Vite build, inject the Semantic_Version, and apply the shared design system and narrow-viewport layout
    - _Requirements: 15.5, 15.6, 15.9_

  - [x] 22.2 Build the `gomobile` AAR and the Capacitor bridge plugin
    - Compile `httpcore`/`store` to an `.aar` via `gomobile bind`; implement the custom plugin and `capacitorBackend` so requests run through native Go (no WebView CORS) and persist on-device
    - _Requirements: 15.1, 15.2, 15.4, 15.5_

  - [x]* 22.3 Write Android instrumented tests for the bridge
    - Send each HTTP method through the Capacitor↔AAR bridge against no-CORS and error endpoints; assert response display and error preservation
    - _Requirements: 15.1, 15.2, 15.3, 15.4_

  - [x] 22.4 Extend the release workflow to build and publish the Android artifact
    - Build the Android Release_Artifact on semver tags, gate its publish independently so a failed Android step does not block desktop publish and vice versa
    - _Requirements: 15.7, 15.8_

- [x] 23. Final checkpoint - all platforms and pipelines
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for a faster MVP; they cover property-based, unit, integration, smoke, and audit tests.
- Each property test must run a minimum of 100 iterations and be tagged with a comment in the format **Feature: volt-api-client, Property {number}: {property_text}**; each correctness property is implemented by a single property-based test.
- Go core/store properties use `testing/quick` or `gopter` with the network mocked via the injectable `Doer` and an in-memory SQLite store; frontend pure-helper properties use `fast-check` with Vitest.
- Each task references specific requirement clauses for traceability; checkpoints provide incremental validation points.
- Stretch items (OAuth2 3.10, Postman/OpenAPI import 8.8, proxy 9.9) are intentionally excluded from this plan.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["2.1", "3.1", "4.1", "5.1"] },
    { "id": 2, "tasks": ["2.2", "3.2", "3.3", "4.2", "4.3", "5.2"] },
    { "id": 3, "tasks": ["6.1"] },
    { "id": 4, "tasks": ["6.2"] },
    { "id": 5, "tasks": ["6.3", "6.4"] },
    { "id": 6, "tasks": ["8.1"] },
    { "id": 7, "tasks": ["8.2", "8.4", "8.6", "8.8"] },
    { "id": 8, "tasks": ["8.3", "8.5", "8.7", "8.9", "9.1", "9.3", "9.6", "9.8"] },
    { "id": 9, "tasks": ["9.2", "9.4", "9.5", "9.7", "9.9", "10.1"] },
    { "id": 10, "tasks": ["10.2", "10.3", "12.1", "12.2"] },
    { "id": 11, "tasks": ["12.3", "13.1", "13.2"] },
    { "id": 12, "tasks": ["13.3"] },
    { "id": 13, "tasks": ["14.1", "14.3", "15.1", "16.1", "17.1", "18.1"] },
    { "id": 14, "tasks": ["14.2", "14.4", "14.5", "15.2", "15.3", "15.4", "16.2", "17.2", "17.3", "18.2"] },
    { "id": 15, "tasks": ["18.3", "19.1", "19.2", "21.1", "21.2", "22.1"] },
    { "id": 16, "tasks": ["21.3", "22.2", "22.4"] },
    { "id": 17, "tasks": ["22.3"] }
  ]
}
```
