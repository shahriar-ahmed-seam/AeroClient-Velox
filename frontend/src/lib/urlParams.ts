// Pure, dependency-free helpers for keeping a request's raw URL and its query
// parameter table in sync (Requirements 1.7 and 1.8).
//
// Both functions are intentionally PURE: given the same inputs they always
// produce the same output and they mutate nothing. The bidirectional UI sync in
// UrlBar/ParamsTable is built on top of them, and Property 2 (the URL <-> param
// round-trip) is property-tested directly against this module.
//
// The URL is treated as plain text rather than parsed with the WHATWG `URL`
// constructor, because the editor URL may be incomplete while the user types
// (no scheme yet, a relative path, or unresolved `{{variable}}` tokens). We only
// ever touch the query-string segment, leaving the base/path and any fragment
// exactly as written.

import type { KeyValue } from './models'

/**
 * Split a raw URL into its base (scheme + host + path), the query string
 * (without the leading `?`), and the fragment (including the leading `#`).
 * Anything the user typed outside the query is preserved verbatim.
 */
function splitUrl(url: string): { base: string; query: string; fragment: string } {
  // Peel off the fragment first so a `?` inside a fragment is not mistaken for
  // the query delimiter.
  const hashIndex = url.indexOf('#')
  const fragment = hashIndex >= 0 ? url.slice(hashIndex) : ''
  const beforeFragment = hashIndex >= 0 ? url.slice(0, hashIndex) : url

  const queryIndex = beforeFragment.indexOf('?')
  const base = queryIndex >= 0 ? beforeFragment.slice(0, queryIndex) : beforeFragment
  const query = queryIndex >= 0 ? beforeFragment.slice(queryIndex + 1) : ''

  return { base, query, fragment }
}

/**
 * Extract the query parameters present in a URL's query string as an enabled
 * KeyValue table (Requirement 1.7). Each surviving `name=value` pair becomes one
 * enabled row with its key and value percent-decoded. Empty segments produced by
 * stray `&` separators are ignored. A value-less segment (`name`) decodes to an
 * empty value, and a `name=` segment likewise yields an empty value.
 */
export function parseQueryToParams(url: string): KeyValue[] {
  const { query } = splitUrl(url)
  if (query === '') return []

  const params: KeyValue[] = []
  for (const segment of query.split('&')) {
    if (segment === '') continue // skip empty pieces from leading/double/trailing `&`

    // Split on the first `=` only; encoded `=` inside a value survives as %3D and
    // is restored by decoding, so the raw split is unambiguous.
    const eqIndex = segment.indexOf('=')
    const rawKey = eqIndex >= 0 ? segment.slice(0, eqIndex) : segment
    const rawValue = eqIndex >= 0 ? segment.slice(eqIndex + 1) : ''

    params.push({
      key: safeDecode(rawKey),
      value: safeDecode(rawValue),
      enabled: true,
    })
  }
  return params
}

/**
 * Rebuild a URL's query string from the enabled, non-empty-key parameters
 * (Requirement 1.8), preserving the base/path and any fragment. Disabled rows and
 * rows with an empty key are excluded. Keys and values are percent-encoded so the
 * produced URL round-trips back through `parseQueryToParams` to an equivalent
 * table. When no parameter qualifies, the `?` is dropped entirely.
 */
export function applyParamsToUrl(url: string, params: KeyValue[]): string {
  const { base, fragment } = splitUrl(url)

  const query = params
    .filter((p) => p.enabled && p.key !== '')
    .map((p) => `${encodeURIComponent(p.key)}=${encodeURIComponent(p.value)}`)
    .join('&')

  return query === '' ? base + fragment : `${base}?${query}${fragment}`
}

/**
 * Decode a percent-encoded query component, falling back to the raw text if it
 * contains an invalid escape sequence (so a half-typed `%` never throws).
 */
function safeDecode(value: string): string {
  try {
    return decodeURIComponent(value)
  } catch {
    return value
  }
}
