// @vitest-environment jsdom
//
// Property test for HeadersView (task 15.3).
//
// HeadersView renders the response headers as name/value rows in the exact
// order the engine returned them. The component iterates the `headers` slice
// as-is — no sorting, no reordering, no dedup — so for any array of KeyValue
// rows (including duplicate keys) the rendered sequence of (key, value) pairs
// must equal the input sequence.
//
// This is the first component test in the suite, so it opts into jsdom via the
// `// @vitest-environment jsdom` header above; the global Vitest environment
// stays 'node' to keep the pure-logic tests fast.
//
// Validates: Requirements 4.8

import {describe, it, expect, afterEach} from 'vitest'
import {render, cleanup} from '@testing-library/svelte'
import fc from 'fast-check'
import type {KeyValue} from '../../lib/models'
import HeadersView from './HeadersView.svelte'

afterEach(() => {
  cleanup()
})

// An arbitrary header row: arbitrary key/value strings (which may collide to
// produce duplicate header names) and an arbitrary enabled flag (HeadersView
// renders every row regardless of `enabled`).
const headerArb: fc.Arbitrary<KeyValue> = fc.record({
  key: fc.string(),
  value: fc.string(),
  enabled: fc.boolean(),
})

// Read the rendered (key, value) sequence out of the DOM in document order.
function renderedPairs(container: HTMLElement): Array<{key: string; value: string}> {
  return Array.from(container.querySelectorAll('.row')).map((row) => ({
    key: row.querySelector('.hk')?.textContent ?? '',
    value: row.querySelector('.hv')?.textContent ?? '',
  }))
}

describe('HeadersView order preservation', () => {
  // Feature: volt-api-client, Property 10: Response headers preserve received order
  it('renders header name/value pairs in exactly the input order', () => {
    fc.assert(
      fc.property(fc.array(headerArb), (headers) => {
        const {container} = render(HeadersView, {props: {headers}})
        try {
          const expected = headers.map((h) => ({key: h.key, value: h.value}))
          expect(renderedPairs(container)).toEqual(expected)
        } finally {
          cleanup()
        }
      }),
      {numRuns: 100},
    )
  })
})
