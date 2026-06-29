# Requirements Document

## Introduction

Volt is a lightweight, cross-platform API client (a Postman/Hoppscotch alternative) built on a Wails v2 desktop shell with a Go-native HTTP engine and a Svelte 5 frontend. A working prototype already exists: a Go HTTP execution engine that bypasses browser CORS, a Svelte UI (method dropdown, URL bar, Send, Parameters/Body/Headers/Authorization tabs, a response panel, Bearer/Basic auth, and localStorage-based history), and a functioning `wails build` pipeline.

This specification defines the requirements to evolve Volt from a prototype into an end-to-end deployable product. The scope covers five pillars:

1. **Distribution** — releasable builds published to GitHub Releases with automated CI/CD and semantic versioning.
2. **Frontend design quality** — a professional, polished, fully-featured UI with generous spacing and strictly square corners (no rounded corners anywhere), where every advertised feature is fully functional.
3. **Responsive design** — a layout that adapts from wide desktop down to narrow viewports using collapsible panels, bottom sheets, and stacked/swipeable tabs.
4. **Android availability** — a working Android API client app released on GitHub.
5. **Complete API client feature set** — request building, request bodies, full authorization options, a rich response viewer, persistent collections, environments and variable interpolation, persistent history, import/export, settings, and keyboard shortcuts with a command palette.

> **Open technical decision (for the Design phase):** Wails v2 is desktop-only and cannot produce Android builds. The Android requirements in this document are written as user-facing outcomes. The Design phase must resolve the technical approach (for example: a shared Go HTTP core compiled via `gomobile`, a separate mobile frontend reusing the Svelte UI inside a Capacitor or Tauri-mobile shell, or a thin embedded Go engine). The requirement is the outcome — a working, GitHub-released Android API client — not a specific mechanism.

## Glossary

- **Volt**: The overall API client product, including desktop and Android distributions.
- **Desktop_App**: The Wails v2 + Svelte 5 desktop build of Volt for Windows, macOS, and Linux.
- **Android_App**: The Android distribution of Volt.
- **HTTP_Engine**: The Go-native request execution component (`ExecuteRequest`) that performs HTTP calls outside the webview, free of browser CORS restrictions.
- **Frontend**: The Svelte 5 user interface layer.
- **Request**: A user-defined HTTP call consisting of method, URL, query parameters, headers, body, and authorization configuration.
- **Response_Viewer**: The UI component that displays response status, timing, size, headers, and body.
- **Collection**: A named, ordered grouping of saved Requests, optionally nested in Folders.
- **Folder**: A nestable container for Requests and other Folders inside a Collection.
- **Environment**: A named set of Variables that can be activated to supply values for interpolation.
- **Variable**: A named value, referenced in Requests using the `{{name}}` syntax.
- **Variable_Interpolation**: The substitution of `{{name}}` tokens with the resolved Variable value at request-execution time.
- **History**: The chronological, persisted record of executed Requests.
- **Persistence_Store**: The local durable storage managed by the Go backend (SQLite database or local file store) for Collections, Environments, History, and Settings.
- **Command_Palette**: A keyboard-invoked overlay for searching and executing application commands.
- **Settings**: User-configurable application preferences (theme, TLS verification, request timeout, proxy).
- **CI_Pipeline**: The automated GitHub Actions build, test, and release workflow.
- **Release_Artifact**: A built, distributable binary or package attached to a GitHub Release.
- **Semantic_Version**: A version identifier following the `MAJOR.MINOR.PATCH` format.
- **Design_System**: The shared set of styling rules (spacing scale, square-corner rule, typography, color tokens) applied across the Frontend.

## Requirements

### Requirement 1: Request Building

**User Story:** As an API developer, I want to construct HTTP requests with any method, URL, query parameters, and headers, so that I can call any endpoint I need to test.

#### Acceptance Criteria

1. THE Frontend SHALL provide method selection for GET, POST, PUT, PATCH, DELETE, HEAD, and OPTIONS, with GET selected by default.
2. WHEN a user enters a URL and activates Send, THE HTTP_Engine SHALL execute the Request using the selected method.
3. IF the URL is empty, omits a scheme, uses a scheme other than http or https, or omits a host, THEN THE HTTP_Engine SHALL return a validation error that identifies the failed validation rule and SHALL NOT perform a network call.
4. WHEN a user adds enabled, non-empty query parameters, THE HTTP_Engine SHALL append each as a percent-encoded name-value pair to the request URL query string.
5. WHEN a user disables a query parameter or header row, THE HTTP_Engine SHALL exclude that row from the executed Request.
6. WHEN a user adds enabled, non-empty headers, THE HTTP_Engine SHALL set each on the executed Request, preserving all rows that share a header name.
7. WHEN a user edits the raw URL query string, THE Frontend SHALL update the parameters table to reflect the query parameters present in the URL.
8. WHEN a user edits the parameters table, THE Frontend SHALL update the raw URL query string to reflect the enabled parameters.

### Requirement 2: Request Bodies

**User Story:** As an API developer, I want to send different request body formats, so that I can match the content type each endpoint expects.

#### Acceptance Criteria

1. THE Frontend SHALL provide body type selection for None, Raw JSON, Plain Text, form-data, and x-www-form-urlencoded, with None selected by default.
2. WHEN the body type is None, THE HTTP_Engine SHALL execute the Request without a body and without setting a Content-Type header.
3. WHEN the body type is Raw JSON and no Content-Type header is set by the user, THE HTTP_Engine SHALL set the Content-Type header to `application/json`.
4. WHEN the body type is Plain Text and no Content-Type header is set by the user, THE HTTP_Engine SHALL set the Content-Type header to `text/plain`.
5. WHERE the body type is Raw JSON or Plain Text and the user has set a Content-Type header, THE HTTP_Engine SHALL use the user-set Content-Type header.
6. WHEN the body type is x-www-form-urlencoded, THE HTTP_Engine SHALL encode the enabled, non-empty-key pairs as a URL-encoded form body and set the Content-Type header to `application/x-www-form-urlencoded`.
7. WHEN the body type is form-data, THE HTTP_Engine SHALL encode the enabled, non-empty-key pairs as a multipart form body and set the Content-Type header to the multipart type with the engine-generated boundary.
8. WHEN encoding a form body, THE HTTP_Engine SHALL exclude any disabled row and any row with an empty key.
9. WHEN the method is GET or HEAD, THE HTTP_Engine SHALL execute the Request without a request body regardless of the selected body type.
10. WHEN the body type is Raw JSON and the user activates Prettify, THE Frontend SHALL reformat valid JSON with two-space indentation within 1 second.
11. IF the body type is Raw JSON and the content is not valid JSON when Prettify is activated, THEN THE Frontend SHALL leave the body content unchanged and display an invalid-JSON indication.

### Requirement 3: Authorization

**User Story:** As an API developer, I want to configure authorization per request, so that I can access protected endpoints without manually crafting auth headers.

#### Acceptance Criteria

1. THE Frontend SHALL provide authorization type selection for None, Bearer Token, Basic Auth, and API Key, with exactly one type active at a time.
2. WHEN the authorization type is Bearer Token and the token contains at least one non-whitespace character, THE HTTP_Engine SHALL send an `Authorization` header with the value `Bearer <token>`.
3. IF the authorization type is Bearer Token and the token is empty or contains only whitespace, THEN THE HTTP_Engine SHALL execute the Request without adding an `Authorization` header.
4. WHEN the authorization type is Basic Auth and the username contains at least one non-whitespace character, THE HTTP_Engine SHALL send an `Authorization` header with the value `Basic ` followed by the Base64 encoding of `<username>:<password>`, treating a missing password as an empty string.
5. WHERE the authorization type is API Key with location Header and the key name contains at least one non-whitespace character, THE HTTP_Engine SHALL send the configured key name and value as a request header.
6. WHERE the authorization type is API Key with location Query and the key name contains at least one non-whitespace character, THE HTTP_Engine SHALL append the configured key name and value to the request URL query string as a single name-value pair.
7. IF the authorization type is API Key and the key name is empty or contains only whitespace, THEN THE HTTP_Engine SHALL execute the Request without adding the API key.
8. WHEN the authorization type is None, THE HTTP_Engine SHALL execute the Request without adding any authorization header or query parameter.
9. WHERE an authorization type produces an `Authorization` header, THE HTTP_Engine SHALL use that value in place of any `Authorization` header otherwise configured in the headers table.
10. WHERE OAuth2 authorization is configured, THE Frontend SHALL obtain an access token and THE HTTP_Engine SHALL send the token in the `Authorization` header. (Stretch)

### Requirement 4: Response Viewer

**User Story:** As an API developer, I want to inspect the full response with status, timing, size, headers, and a readable body, so that I can verify endpoint behavior.

#### Acceptance Criteria

1. WHEN a Request completes and the HTTP_Engine returns a response, THE Response_Viewer SHALL display the numeric status code (100 to 599), the status text, the elapsed time as a whole number of milliseconds, and the response body size in bytes.
2. THE Response_Viewer SHALL display the response status using a visually distinct color for each status class, with one color for 2xx, one for 3xx, one for 4xx, and one for 5xx.
3. THE Response_Viewer SHALL provide Pretty, Raw, and Preview views of the response body, with exactly one view active at a time.
4. WHEN the response body is valid JSON and the Pretty view is active, THE Response_Viewer SHALL display the body with syntax highlighting and two-space indentation.
5. IF the Pretty view is active and the response body is not valid JSON, THEN THE Response_Viewer SHALL display the unmodified body as plain text without syntax highlighting.
6. WHEN the Preview view is active and the response Content-Type is HTML, THE Response_Viewer SHALL render the body as a preview.
7. WHEN the Preview view is active and the response Content-Type is not a previewable type, THE Response_Viewer SHALL display the unmodified body as plain text.
8. THE Response_Viewer SHALL display all response headers as name-value pairs in a Headers view, preserving the order received from the HTTP_Engine.
9. IF the HTTP_Engine returns an error, THEN THE Response_Viewer SHALL display an error message describing the failure in place of a response body and SHALL NOT display a status code, elapsed time, or size.
10. WHILE a Request is in flight, THE Response_Viewer SHALL display a loading indicator.
11. WHEN a response body exceeds 5 megabytes (5,242,880 bytes), THE Response_Viewer SHALL display only the first 5 megabytes of the body together with a control to view or save the complete body.
12. WHEN a user activates the copy control, THE Response_Viewer SHALL copy the currently displayed response body to the system clipboard and display a confirmation indication.

### Requirement 5: Collections

**User Story:** As an API developer, I want to save requests and organize them into collections and folders, so that I can reuse and structure my work.

#### Acceptance Criteria

1. WHEN a user saves a Request to a target Collection or Folder with a name, THE Persistence_Store SHALL store the Request including its name, method, URL, query parameters, headers, body, and authorization configuration.
2. THE Frontend SHALL allow a user to create, rename, and delete a Collection, with Collection names between 1 and 255 characters.
3. THE Frontend SHALL allow a user to create, rename, and delete a Folder within a Collection or within another Folder, supporting nesting up to 10 levels deep.
4. WHEN a user moves a Request into a Collection or Folder, THE Frontend SHALL place the Request in the target location and remove it from its prior location.
5. WHEN a user selects a saved Request, THE Frontend SHALL replace the request editor contents with the stored name, method, URL, query parameters, headers, body, and authorization configuration of that Request.
6. WHEN the Desktop_App restarts, THE Persistence_Store SHALL retain all previously saved Collections, Folders, and Requests, including their names, nesting structure, and order.
7. IF a delete action targets a Collection or Folder that contains Requests, THEN THE Frontend SHALL prompt for confirmation, SHALL delete the container and its contents when the user confirms, and SHALL leave it unchanged when the user declines.
8. IF a user enters a Collection, Folder, or Request name that is empty or exceeds 255 characters, THEN THE Frontend SHALL reject the name and SHALL NOT create or rename the item.
9. IF the Persistence_Store fails to save or delete an item, THEN THE Frontend SHALL display an error and SHALL preserve the existing stored data.

### Requirement 6: Environments and Variables

**User Story:** As an API developer, I want to define environment variables and reference them in my requests, so that I can switch between contexts like dev and prod without editing each request.

#### Acceptance Criteria

1. THE Frontend SHALL allow a user to create, rename, and delete an Environment, with Environment names between 1 and 64 characters and unique among Environments.
2. THE Frontend SHALL allow a user to define, edit, and remove Variables within an Environment, with Variable names between 1 and 128 characters and unique within the Environment, and Variable values between 0 and 4096 characters.
3. THE Frontend SHALL allow a user to select one active Environment at a time.
4. WHEN a Request is executed and a field contains a `{{name}}` token, THE HTTP_Engine SHALL replace every matching token with the value of the case-sensitively matching Variable from the active Environment before sending the Request.
5. THE Variable_Interpolation SHALL apply to the URL, query parameters, headers, and request body.
6. IF a `{{name}}` token references a Variable that is not defined in the active Environment, or no Environment is active, THEN THE Frontend SHALL display an unresolved-variable indication for that token.
7. WHEN the Desktop_App restarts, THE Persistence_Store SHALL retain all Environments and their Variables.
8. IF a `{{name}}` token is unresolved when a Request is executed, THEN THE HTTP_Engine SHALL send the token text unchanged and SHALL still execute the Request.
9. IF a user enters an Environment or Variable name that is empty or duplicates an existing name in the same scope, THEN THE Frontend SHALL reject the name and SHALL preserve the existing data.
10. WHEN a user deletes the active Environment, THE Frontend SHALL set no Environment as active.

### Requirement 7: History

**User Story:** As an API developer, I want a persistent history of the requests I have sent, so that I can review and re-run past calls.

#### Acceptance Criteria

1. WHEN a Request completes successfully, THE Persistence_Store SHALL record a History entry containing the method, URL, status code, elapsed time, timestamp, and the full Request configuration.
2. WHEN a Request fails to complete, THE Persistence_Store SHALL record a History entry containing the method, URL, error indication, timestamp, and the full Request configuration.
3. THE Frontend SHALL display History entries in reverse chronological order.
4. WHEN a user selects a History entry, THE Frontend SHALL restore the stored method, URL, query parameters, headers, body, and authorization configuration into the request editor.
5. WHEN the Desktop_App restarts, THE Persistence_Store SHALL retain all History entries.
6. WHEN the number of History entries exceeds 1000, THE Persistence_Store SHALL discard the oldest entries so that no more than 1000 are retained.
7. WHEN a user activates Clear History, THE Frontend SHALL prompt for confirmation and SHALL remove all History entries only when the user confirms.
8. WHEN the upgraded Desktop_App launches for the first time and localStorage-based History exists, THE Persistence_Store SHALL migrate that History exactly once.
9. IF the History migration fails, THEN THE Persistence_Store SHALL preserve the localStorage-based History and THE Frontend SHALL display an error.

### Requirement 8: Import and Export

**User Story:** As an API developer, I want to import and export my collections and environments, so that I can back up my work and share it across machines.

#### Acceptance Criteria

1. WHEN a user exports a Collection, THE Frontend SHALL write a file in a documented JSON format that contains the Collection, every nested Folder, and every Request including its method, URL, query parameters, headers, body, and authorization configuration, together with a format version identifier.
2. WHEN a user exports an Environment, THE Frontend SHALL write a file in a documented JSON format that contains the Environment and every Variable including its name and value, together with a format version identifier.
3. WHEN a user imports a file previously exported by Volt, THE Frontend SHALL recreate every contained Collection, Folder, Request, and Environment as new entries in the Persistence_Store.
4. WHEN a file exported by Volt is subsequently imported, THE Frontend SHALL reproduce each Collection, Folder, Request, Environment, and Variable with field values and nesting structure identical to those present at export time.
5. IF an imported file is not valid JSON, is structurally malformed, or carries a format version identifier that does not match a supported schema, THEN THE Frontend SHALL reject the entire import, leave all existing Collections, Environments, and Variables unchanged, and display an error message indicating the reason for rejection.
6. IF an imported Collection or Environment has a name that matches an existing Collection or Environment, THEN THE Frontend SHALL import it as a separate new entry without overwriting or modifying the existing entry.
7. IF the Frontend cannot write an export file because the chosen destination is not writable, THEN THE Frontend SHALL abort the export without creating a partial file and display an error message indicating the failure.
8. WHERE a Postman Collection or OpenAPI document is imported, THE Frontend SHALL convert it into Volt Collections and Requests. (Stretch)

### Requirement 9: Settings

**User Story:** As a user, I want to configure application preferences, so that Volt behaves the way I need across different environments.

#### Acceptance Criteria

1. THE Frontend SHALL provide a Settings interface for theme selection (Light, Dark, System), TLS verification, and a request timeout specified as an integer from 1 to 600 seconds.
2. WHEN a user selects a theme, THE Frontend SHALL apply the selected theme across all views within 1 second without requiring a restart.
3. WHILE TLS verification is disabled in Settings, THE HTTP_Engine SHALL skip TLS certificate verification for executed Requests.
4. WHILE TLS verification is disabled in Settings, THE Frontend SHALL display a persistent warning indication.
5. WHEN a Request exceeds the configured request timeout, THE HTTP_Engine SHALL abort the Request and return a timeout error indication.
6. IF a user enters a request timeout outside the range 1 to 600 seconds, THEN THE Frontend SHALL reject the value and retain the previous timeout.
7. WHEN the Desktop_App restarts, THE Persistence_Store SHALL retain all Settings.
8. WHEN the Desktop_App launches for the first time, THE Settings SHALL default to System theme, TLS verification enabled, and a 30-second timeout.
9. WHERE a proxy is configured in Settings, THE HTTP_Engine SHALL route executed Requests through the configured proxy. (Stretch)

### Requirement 10: Keyboard Shortcuts and Command Palette

**User Story:** As a power user, I want keyboard shortcuts and a command palette, so that I can work quickly without reaching for the mouse.

#### Acceptance Criteria

1. WHEN a user presses Ctrl+Enter or Cmd+Enter, THE Frontend SHALL execute the current Request and SHALL suppress the default browser handling of the key combination.
2. WHEN a user presses Ctrl+K or Cmd+K, THE Frontend SHALL open the Command_Palette and SHALL suppress the default browser handling of the key combination.
3. WHILE the Command_Palette is open and a user types a query, THE Command_Palette SHALL display, within 100 milliseconds, the commands whose displayed name contains the query as a case-insensitive substring, and SHALL display all commands when the query is empty.
4. WHEN a user selects a command by pressing Enter on the highlighted entry or by pointer selection, THE Frontend SHALL execute the selected command and close the Command_Palette.
5. WHEN a user presses Ctrl+S or Cmd+S, THE Frontend SHALL save the current Request and SHALL suppress the default browser handling of the key combination.
6. THE Settings interface SHALL display each available keyboard shortcut with its key combination and the command it triggers.
7. IF a typed query matches no command, THEN THE Command_Palette SHALL display a no-results indication and SHALL NOT execute any command.
8. WHEN a user presses Escape while the Command_Palette is open, THE Frontend SHALL close the Command_Palette without executing a command.
9. WHEN a user presses the Up or Down arrow keys while the Command_Palette is open, THE Command_Palette SHALL move the highlight within the filtered list.

### Requirement 11: Professional Frontend Design System

**User Story:** As a user, I want a polished, professional interface with consistent spacing and sharp corners, so that Volt feels like a finished, trustworthy product.

#### Acceptance Criteria

1. THE Frontend SHALL render every visual surface — including components, buttons, inputs, dropdowns, menus, panels, list items, modals, overlays, the Command_Palette, tabs, and bottom sheets — with a `border-radius` of exactly 0px, with no non-zero exceptions.
2. THE Design_System SHALL define a spacing scale as discrete steps that are integer multiples of a single base unit (for example a 4px base, yielding 4, 8, 12, 16, 24, 32, 48), and THE Frontend SHALL draw all padding, margin, and gap values exclusively from that scale.
3. THE Frontend SHALL apply a minimum interior padding of 16px, drawn from the spacing scale, to all container-level surfaces.
4. THE Frontend SHALL apply a single consistent Design_System using named color and typography tokens across all views, with no hardcoded color or typography values.
5. THE Frontend SHALL provide a fully functional user interface for every feature defined in Requirements 1 through 10 and Requirement 12, with no placeholder text, permanently disabled controls, or non-operational controls.
6. WHEN a user activates a displayed control, THE Frontend SHALL perform the control's associated operation and produce an observable result within 200 milliseconds.
7. IF a control's operation fails, THEN THE Frontend SHALL display an error indication and SHALL preserve the user's input.

### Requirement 12: Responsive Layout

**User Story:** As a user, I want the interface to adapt to different window and screen sizes, so that Volt remains usable on both wide desktop displays and narrow viewports.

#### Acceptance Criteria

1. WHILE the viewport width is at or above the Wide breakpoint (1024px), THE Frontend SHALL display the navigation rail, request editor, response viewer, and sidebar simultaneously in a multi-column layout with no horizontal scrolling required to reach any region.
2. WHILE the viewport width is in the Medium range (600px to 1023px), THE Frontend SHALL display the request editor and response viewer while collapsing the sidebar into a toggleable panel.
3. WHILE the viewport width is below the Narrow breakpoint (600px), THE Frontend SHALL collapse secondary panels into collapsible panels or bottom sheets.
4. WHILE the viewport width is below the Narrow breakpoint (600px), THE Frontend SHALL present the request configuration tabs and response tabs as stacked or swipeable tabs that display one tab panel at a time with navigation controls.
5. WHEN the viewport is resized across a breakpoint, THE Frontend SHALL reflow the layout within 500 milliseconds, and every feature reachable before the resize SHALL remain reachable after.
6. THE Frontend SHALL render all interactive controls within the viewport and keep them operable by pointer, keyboard, and touch at every viewport width from 320px to 3840px, with no control clipped or positioned off-screen.

### Requirement 13: Desktop Packaging and Distribution

**User Story:** As a maintainer, I want the desktop app packaged and published with versioned artifacts, so that users can download and install official releases.

#### Acceptance Criteria

1. THE Desktop_App SHALL carry a Semantic_Version identifier in `MAJOR.MINOR.PATCH` format and SHALL display that identifier in a user-accessible location (such as an About or Settings view).
2. WHEN a release build is produced, THE CI_Pipeline SHALL generate one installable Release_Artifact for each of Windows, macOS, and Linux, and each Release_Artifact filename SHALL include the Semantic_Version of the build.
3. WHEN a version tag matching the Semantic_Version format is pushed to the repository, THE CI_Pipeline SHALL build the Desktop_App for Windows, macOS, and Linux and publish a single GitHub Release containing all three Release_Artifacts.
4. WHEN the CI_Pipeline publishes a GitHub Release, THE GitHub Release SHALL be labeled with the Semantic_Version that matches the pushed version tag and SHALL include release notes describing the changes included in that version.
5. IF any build or test step in the CI_Pipeline fails, THEN THE CI_Pipeline SHALL report a failed status for the release run and SHALL NOT publish a GitHub Release or attach any Release_Artifact for that version.
6. IF a pushed version tag does not match the Semantic_Version format, THEN THE CI_Pipeline SHALL NOT start a release build and SHALL NOT publish a GitHub Release for that tag.

### Requirement 14: Continuous Integration

**User Story:** As a maintainer, I want automated builds and checks on changes, so that regressions are caught before release.

#### Acceptance Criteria

1. WHEN a commit is pushed to any branch or a pull request is opened or updated with new commits, THE CI_Pipeline SHALL build both the Go backend and the Svelte Frontend.
2. WHEN the CI_Pipeline runs, THE CI_Pipeline SHALL execute the Go backend automated test suite and the Frontend automated test suite.
3. WHEN the CI_Pipeline runs, THE CI_Pipeline SHALL run the Frontend type-check (`svelte-check`) and SHALL treat any reported type error as a failure.
4. IF any build, type-check, or test step fails, THEN THE CI_Pipeline SHALL report a failed status for the associated commit or pull request that identifies which step failed.
5. WHEN all build, type-check, and test steps complete successfully, THE CI_Pipeline SHALL report a passed status for the associated commit or pull request.

### Requirement 15: Android Application

**User Story:** As a mobile user, I want a Volt Android app, so that I can build and send API requests from my phone.

> The mechanism for producing the Android_App is an open technical decision deferred to the Design phase (see Introduction). The criteria below describe the required user-facing outcomes.

#### Acceptance Criteria

1. THE Android_App SHALL allow a user to build and send a Request using the GET, POST, PUT, PATCH, DELETE, HEAD, and OPTIONS methods.
2. WHEN a user sends a Request from the Android_App, THE Android_App SHALL execute the Request and return a response even when the target host does not return CORS response headers.
3. THE Android_App SHALL display the response status code, status text, elapsed time in milliseconds, size, headers as name-value pairs, and body.
4. IF a Request sent from the Android_App fails due to a network error, timeout, or invalid URL, THEN THE Android_App SHALL display an error indication in place of a response body and SHALL preserve the Request configuration.
5. WHEN the Android_App restarts, THE Android_App SHALL retain all Collections, Environments, and History persisted on the device.
6. THE Android_App SHALL apply the same Design_System as the Desktop_App, including a `border-radius` of 0px on all surfaces and the narrow-viewport responsive layout defined in Requirement 12.
7. WHEN a version tag matching the Semantic_Version format is pushed to the repository, THE CI_Pipeline SHALL build the Android_App and publish the resulting Android Release_Artifact to a GitHub Release.
8. IF any Android build or test step in the CI_Pipeline fails, THEN THE CI_Pipeline SHALL NOT publish the Android Release_Artifact.
9. THE Android_App SHALL carry a Semantic_Version in `MAJOR.MINOR.PATCH` format matching the Desktop_App scheme.
