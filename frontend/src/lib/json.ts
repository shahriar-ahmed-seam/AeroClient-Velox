// Pure, dependency-free JSON helpers shared by the request Body editor and the
// response Pretty view. Keeping this logic pure (no DOM, no stores) means it can
// be exhaustively property-tested in isolation (see tasks 14.4) and reused
// anywhere the same "format if valid, otherwise leave untouched" behavior is
// needed.

/** The outcome of attempting to prettify a string as JSON. */
export interface PrettifyResult {
  /** The reformatted text on success, or the original input unchanged on failure. */
  text: string
  /** True when the input parsed as valid JSON and was reformatted. */
  ok: boolean
}

/**
 * Reformat a string as JSON using two-space indentation.
 *
 * On valid JSON the parsed value is re-serialized with `JSON.stringify(value, null, 2)`
 * and `ok` is true (Req 2.10). On invalid JSON the input is returned unchanged and
 * `ok` is false so callers can surface an invalid-JSON indication (Req 2.11).
 *
 * The function is pure: it never throws and never mutates its argument.
 */
export function prettifyJSON(input: string): PrettifyResult {
  try {
    const parsed = JSON.parse(input)
    return { text: JSON.stringify(parsed, null, 2), ok: true }
  } catch {
    return { text: input, ok: false }
  }
}
