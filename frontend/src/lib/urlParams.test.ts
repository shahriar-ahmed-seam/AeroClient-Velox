// Feature: volt-api-client, Property 2: URL and parameter table round-trip
//
// Property 2 (design.md): "For any set of enabled, non-empty query parameters,
// encoding them into the raw URL query string and then parsing that query
// string back into a parameter table yields a parameter table equivalent to
// the original enabled parameters."
//
// Validates: Requirements 1.7, 1.8
//
// This is the first frontend test; it exercises the pure urlParams helpers that
// back the bidirectional UrlBar <-> ParamsTable sync. The helpers document that
// applyParamsToUrl excludes disabled and empty-key rows, so the round-trip
// property is stated over enabled, non-empty-key rows.

import {describe, it, expect} from 'vitest'
import fc from 'fast-check'
import type {KeyValue} from './models'
import {applyParamsToUrl, parseQueryToParams} from './urlParams'

// A single enabled, non-empty-key parameter row. Keys and values intentionally
// draw from the full (surrogate-safe) Unicode range so they include characters
// that require percent-encoding (spaces, '&', '=', '?', '#', '%', etc.).
const paramArb: fc.Arbitrary<KeyValue> = fc.record({
  key: fc.string({unit: 'grapheme', minLength: 1}),
  value: fc.string({unit: 'grapheme'}),
  enabled: fc.constant(true),
})

// An ordered table of enabled, non-empty-key parameters (duplicate keys allowed,
// so order preservation is meaningfully exercised).
const paramsArb: fc.Arbitrary<KeyValue[]> = fc.array(paramArb, {maxLength: 12})

// A base URL with no query string and no fragment, since applyParamsToUrl
// rewrites the query segment wholesale. Includes the empty string to cover the
// "no base yet typed" editor case.
const baseUrlArb: fc.Arbitrary<string> = fc.oneof(
  fc.constant(''),
  fc.webUrl({withQueryParameters: false, withFragments: false}),
)

describe('Property 2: URL and parameter table round-trip', () => {
  it('encoding enabled params into a URL then parsing back yields the same table (keys, values, order preserved)', () => {
    fc.assert(
      fc.property(baseUrlArb, paramsArb, (base, params) => {
        const url = applyParamsToUrl(base, params)
        const parsed = parseQueryToParams(url)

        // parseQueryToParams always returns enabled rows; our inputs are already
        // enabled and non-empty-key, so the round-trip must be exact.
        expect(parsed).toEqual(
          params.map((p) => ({key: p.key, value: p.value, enabled: true})),
        )
      }),
      {numRuns: 100},
    )
  })

  it('re-applying parsed params reproduces the identical query (URL<->params sync is stable)', () => {
    fc.assert(
      fc.property(baseUrlArb, paramsArb, (base, params) => {
        const url = applyParamsToUrl(base, params)
        const parsed = parseQueryToParams(url)
        const reapplied = applyParamsToUrl(base, parsed)

        // Parsing a URL's query and re-applying it must yield an equivalent
        // (here, byte-identical) query string.
        expect(reapplied).toBe(url)
      }),
      {numRuns: 100},
    )
  })
})
