// Feature: volt-api-client, Property 6: JSON formatting preserves value or leaves invalid input unchanged
//
// Property 6 (design.md): "For any string, if it is valid JSON then Prettify
// produces output that parses to an equal JSON value and is indented with two
// spaces; if it is not valid JSON then Prettify leaves the content unchanged and
// reports an invalid-JSON indication. (The response Pretty view uses the same
// function and therefore shares this property.)"
//
// Validates: Requirements 2.10, 2.11, 4.4, 4.5
//
// prettifyJSON is the single helper shared by the request Body editor (Req 2.10,
// 2.11) and the response Pretty view (Req 4.4, 4.5), so this one property covers
// both surfaces. The two halves of the property are exercised separately:
//   - valid input  -> ok:true, value preserved, canonical two-space indentation
//   - invalid input -> ok:false, text byte-identical to the input (unchanged)

import {describe, it, expect} from 'vitest'
import fc from 'fast-check'
import {prettifyJSON} from './json'

// Arbitrary JSON value drawn from the full JSON grammar (objects, arrays,
// strings, numbers, booleans, null), serialized to a JSON string for input.
const jsonStringArb: fc.Arbitrary<string> = fc
  .jsonValue()
  .map((v) => JSON.stringify(v))

// Strings that are NOT valid JSON. We combine random strings filtered to those
// that fail JSON.parse with a handful of known-invalid fixtures so the space of
// malformed inputs (truncated, trailing junk, single-quoted, bare identifiers)
// is reliably exercised even when the random generator favors short strings.
const invalidJsonArb: fc.Arbitrary<string> = fc.oneof(
  fc.string().filter((s) => {
    try {
      JSON.parse(s)
      return false
    } catch {
      return true
    }
  }),
  fc.constantFrom(
    '',
    '   ',
    '{',
    '}',
    '[',
    '[1, 2',
    '{"a": 1',
    "{'a': 1}",
    '{a: 1}',
    'undefined',
    'NaN',
    '01',
    '1.2.3',
    '{"a": 1,}',
    'tru',
    'nul',
    '"unterminated',
    '<html></html>',
  ),
)

describe('Property 6: JSON formatting preserves value or leaves invalid input unchanged', () => {
  it('reformats valid JSON to two-space indentation while preserving the parsed value (ok:true)', () => {
    fc.assert(
      fc.property(jsonStringArb, (input) => {
        const result = prettifyJSON(input)

        // Valid JSON is reported as such.
        expect(result.ok).toBe(true)

        // The output is itself valid JSON (it parses without throwing) and its
        // parsed value equals the parsed input value -> the value is preserved.
        const reparsed = JSON.parse(result.text)
        expect(reparsed).toEqual(JSON.parse(input))

        // The output is exactly the canonical two-space-indented serialization.
        expect(result.text).toBe(JSON.stringify(JSON.parse(input), null, 2))
      }),
      {numRuns: 100},
    )
  })

  it('leaves invalid JSON unchanged and reports it as invalid (ok:false, text === input)', () => {
    fc.assert(
      fc.property(invalidJsonArb, (input) => {
        const result = prettifyJSON(input)

        // Invalid JSON is flagged so callers can surface an invalid-JSON indication.
        expect(result.ok).toBe(false)

        // The content is returned byte-for-byte unchanged.
        expect(result.text).toBe(input)
      }),
      {numRuns: 100},
    )
  })
})
