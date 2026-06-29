// Feature: volt-api-client, Property 23: Command palette substring filtering
//
// Property 23 (design.md): "For any set of commands and any query string, the
// palette displays exactly the commands whose displayed name contains the query
// as a case-insensitive substring, and displays all commands when the query is
// empty."
//
// Validates: Requirements 10.3, 10.7
//
// These property tests exercise the pure `filterCommands` helper that backs the
// Command Palette. The helper documents its contract as: case-insensitive
// substring match on each command's `title`; an empty or whitespace-only query
// returns all commands; input order is preserved; and the input array is never
// mutated.

import {describe, it, expect} from 'vitest'
import fc from 'fast-check'
import {filterCommands, type Command} from './commands'

// A no-op run callback; `filterCommands` never invokes it, but the Command shape
// requires it.
const noop = () => {}

// A single command with an arbitrary id and title. Titles draw from the full
// (surrogate-safe) Unicode grapheme range so case-folding and substring matching
// are exercised against more than plain ASCII.
const commandArb: fc.Arbitrary<Command> = fc.record({
  id: fc.string({minLength: 1}),
  title: fc.string({unit: 'grapheme'}),
  run: fc.constant(noop),
})

// An ordered list of commands (duplicate titles allowed, so order preservation
// is meaningfully exercised).
const commandsArb: fc.Arbitrary<Command[]> = fc.array(commandArb, {maxLength: 12})

// A query that is biased toward "interesting" values: a substring drawn from one
// of the command titles (so matches actually occur), a randomly-cased version of
// such a substring (to exercise case-insensitivity), and free-form random
// strings (most of which will not match).
function queryArb(commands: Command[]): fc.Arbitrary<string> {
  const titles = commands.map((c) => c.title).filter((t) => t.length > 0)
  const options: fc.Arbitrary<string>[] = [fc.string({unit: 'grapheme'})]
  if (titles.length > 0) {
    // A substring slice [start, end) of a chosen title.
    const substringArb = fc
      .constantFrom(...titles)
      .chain((title) =>
        fc
          .tuple(
            fc.nat({max: title.length}),
            fc.nat({max: title.length}),
          )
          .map(([a, b]) => title.slice(Math.min(a, b), Math.max(a, b))),
      )
    // The same substring with each character coerced toward upper or lower case,
    // so a matching query and the title differ only by case.
    const mixedCaseArb = substringArb.chain((s) =>
      fc
        .array(fc.boolean(), {minLength: s.length, maxLength: s.length})
        .map((flags) =>
          Array.from(s)
            .map((ch, i) => (flags[i] ? ch.toUpperCase() : ch.toLowerCase()))
            .join(''),
        ),
    )
    options.push(substringArb, mixedCaseArb)
  }
  return fc.oneof(...options)
}

// Whitespace-only (including empty) query strings, per Req 10.7: a no-results
// state only arises from a *non-empty* query that matches nothing.
const blankQueryArb: fc.Arbitrary<string> = fc
  .array(fc.constantFrom(' ', '\t', '\n', '\r', '\f', '\v'), {maxLength: 6})
  .map((parts) => parts.join(''))

describe('Property 23: Command palette substring filtering', () => {
  it('returns exactly the case-insensitive substring matches, in input order', () => {
    fc.assert(
      fc.property(
        commandsArb.chain((commands) =>
          fc.tuple(fc.constant(commands), queryArb(commands)),
        ),
        ([commands, query]) => {
          const result = filterCommands(commands, query)
          const needle = query.trim().toLowerCase()

          // The expected subset, computed independently and preserving order.
          const expected =
            needle === ''
              ? commands.slice()
              : commands.filter((c) => c.title.toLowerCase().includes(needle))

          // Result is exactly the matching subset, order preserved. Comparing the
          // full array (not just membership) verifies both inclusion of every
          // match and exclusion of every non-match, with order intact.
          expect(result).toEqual(expected)

          // Cross-check the membership characterization directly: every command
          // in the result matches; every command not in the result does not.
          for (const c of commands) {
            const included = result.includes(c)
            const matches =
              needle === '' || c.title.toLowerCase().includes(needle)
            expect(included).toBe(matches)
          }
        },
      ),
      {numRuns: 100},
    )
  })

  it('returns all commands for an empty or whitespace-only query (Req 10.7)', () => {
    fc.assert(
      fc.property(commandsArb, blankQueryArb, (commands, query) => {
        const result = filterCommands(commands, query)
        expect(result).toEqual(commands)
      }),
      {numRuns: 100},
    )
  })

  it('preserves input order as a subsequence and never mutates the input', () => {
    fc.assert(
      fc.property(
        commandsArb.chain((commands) =>
          fc.tuple(fc.constant(commands), queryArb(commands)),
        ),
        ([commands, query]) => {
          const snapshot = commands.slice()
          const result = filterCommands(commands, query)

          // Input array is unchanged (same length, same elements, same order).
          expect(commands).toEqual(snapshot)

          // Result order is a subsequence of the input order: walking the input
          // once must encounter every result element in result order.
          let cursor = 0
          for (const c of commands) {
            if (cursor < result.length && result[cursor] === c) cursor++
          }
          expect(cursor).toBe(result.length)
        },
      ),
      {numRuns: 100},
    )
  })
})
