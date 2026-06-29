// Small deep-clone helper shared by the stores. The editor and history/saved
// configurations must be cloned before being loaded into reactive `$state` so
// that mutating the editor never writes back through a shared reference into
// the persisted tree or history list. The Wails (Chromium) and Capacitor
// (Android WebView) runtimes both provide structuredClone; the JSON fallback
// keeps the helper safe in any test/SSR environment where it is absent.

export function deepClone<T>(value: T): T {
  const sc = (globalThis as { structuredClone?: <U>(v: U) => U }).structuredClone
  if (typeof sc === 'function') {
    return sc(value)
  }
  return JSON.parse(JSON.stringify(value)) as T
}
