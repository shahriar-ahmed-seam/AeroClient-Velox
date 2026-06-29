# Volt frontend

The Svelte 5 single-page UI for Volt. It is platform-agnostic: the same build is
embedded by the Wails desktop app and the Capacitor Android shell, talking to the
shared Go core only through the `Backend` interface in `src/lib/backend/`.

> For the project overview, architecture, and release process see the
> [root README](../README.md).

## Stack

- **Svelte 5** (runes: `$state` / `$derived` / `$effect`) + **TypeScript**
- **Vite** for dev/build
- **Vitest** + **fast-check** for unit and property-based tests (jsdom for components)

## Scripts

```bash
npm install        # install dependencies
npm run dev        # Vite dev server (hot reload)
npm run build      # production build → dist/ (embedded by Wails / Capacitor)
npm run check      # svelte-check type-check (CI gate; must be 0 errors)
npm test           # Vitest run (unit + property tests)
npm run test:watch # Vitest watch mode
```

### Android (Capacitor)

```bash
npm run cap:add:android      # generate the native android/ project (needs Android SDK)
npm run cap:sync             # copy dist/ + plugins into android/
npm run android:set-version  # stamp versionName/versionCode from a semver
npm run android:build        # build + sync in one step
```

## Layout

```
src/
├── lib/
│   ├── models.ts            # TypeScript mirrors of internal/model (camelCase JSON)
│   ├── backend/             # Backend interface + wailsBackend / capacitorBackend
│   ├── stores/              # Svelte 5 runes stores (request, collections, env, …)
│   ├── native/              # Capacitor VoltBridge plugin definition
│   └── urlParams.ts, json.ts, commands.ts, shortcuts.ts   # pure helpers
├── components/
│   ├── request/             # UrlBar, ParamsTable, HeadersTable, AuthPanel, BodyEditor
│   ├── response/            # ResponseViewer, StatusBar, BodyView, HeadersView
│   ├── sidebar/             # CollectionsTree, HistoryList, EnvironmentManager
│   ├── command/             # CommandPalette
│   └── settings/            # SettingsView
├── styles/tokens.css        # design-system tokens (color, type, spacing, radius)
├── App.svelte               # responsive shell wiring everything together
└── main.ts                  # mount point
```

## Conventions

- All persistent mutations go through `getBackend()` — components never touch the
  Go layer directly.
- Styling uses design-system tokens (`var(--space-*)`, `var(--color-*)`, …); the
  global `border-radius: 0` rule is enforced by `styles/tokens.css` and audited in
  `src/design-system.audit.test.ts`.
- Pure helpers are kept separate from components so they can be property-tested in
  isolation.
