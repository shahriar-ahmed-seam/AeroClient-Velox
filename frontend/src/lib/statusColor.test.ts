// Feature: volt-api-client, Property 9: Response status color matches its status class
//
// Property 9 (design.md): "Response status color matches its status class."
// For any integer status code, statusColor returns exactly the color token for
// its status class: 200-299 -> var(--green), 300-399 -> var(--blue),
// 400-499 -> var(--accent), 500-599 -> var(--red), everything else -> var(--text-dim).
//
// Validates: Requirements 4.2
//
// This exercises the pure statusColor helper in ./types. The oracle below encodes
// the status-class rules from the property (not the implementation), so the test
// asserts the helper agrees with the specified per-class color tokens. Generators
// span every class and the surrounding boundaries (199/200/299/300/399/400/499/
// 500/599/600) plus out-of-range values to stress the class edges.

import {describe, it, expect} from 'vitest'
import fc from 'fast-check'
import {statusColor} from './types'

// Exact color tokens returned by statusColor for each status class.
const GREEN = 'var(--green)'
const BLUE = 'var(--blue)'
const ACCENT = 'var(--accent)'
const RED = 'var(--red)'
const TEXT_DIM = 'var(--text-dim)'

// Oracle: the expected color for a status code per the Property 9 class rules.
function expectedColor(status: number): string {
  if (status >= 200 && status <= 299) return GREEN
  if (status >= 300 && status <= 399) return BLUE
  if (status >= 400 && status <= 499) return ACCENT
  if (status >= 500 && status <= 599) return RED
  return TEXT_DIM
}

// Integer status codes spanning all classes, the exact boundaries between
// classes, and out-of-range values (negative, sub-2xx, and >= 600).
const statusArb: fc.Arbitrary<number> = fc.oneof(
  // Boundary values around each class edge.
  fc.constantFrom(199, 200, 299, 300, 399, 400, 499, 500, 599, 600),
  // Out-of-range / unusual values.
  fc.constantFrom(0, 1, 99, 100, 601, 700, 999, -1, -200),
  // Broad coverage across and beyond the HTTP status range.
  fc.integer({min: -1000, max: 2000}),
)

describe('Property 9: Response status color matches its status class', () => {
  it('statusColor returns the exact color token for the status code class', () => {
    fc.assert(
      fc.property(statusArb, (status) => {
        expect(statusColor(status)).toBe(expectedColor(status))
      }),
      {numRuns: 100},
    )
  })
})
