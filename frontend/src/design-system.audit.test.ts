// @vitest-environment node
//
// Automated Design-System Audit (Task 19.1)
// -----------------------------------------
// Static-analysis tests that read the frontend source from disk and assert the
// Design_System rules from Requirement 11 hold across every styled file. These
// tests do not render components; they scan `.svelte` and `.css` source text so
// that any future drift away from the token layer fails CI.
//
// Validates: Requirements 11.1, 11.2, 11.3, 11.4

import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join, relative, sep } from 'node:path'

// This test file lives at <frontend>/src/design-system.audit.test.ts, so its
// directory IS the src root we want to walk. Resolving from import.meta.url
// makes the audit independent of the process working directory.
const SRC_DIR = dirname(fileURLToPath(import.meta.url))
const TOKENS_FILE = join(SRC_DIR, 'styles', 'tokens.css')
const STYLE_FILE = join(SRC_DIR, 'style.css')

/** Recursively collect files under `dir` whose name matches one of `exts`. */
function walk(dir: string, exts: string[]): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    const st = statSync(full)
    if (st.isDirectory()) {
      if (entry === 'node_modules' || entry === 'dist' || entry === 'assets') continue
      out.push(...walk(full, exts))
    } else if (exts.some((e) => entry.endsWith(e))) {
      out.push(full)
    }
  }
  return out
}

const read = (p: string) => readFileSync(p, 'utf8')
const rel = (p: string) => relative(SRC_DIR, p).split(sep).join('/')

/**
 * Strip comments so prose mentioning style properties (e.g. a `// inherits the
 * border-radius:0 rule` note) is never mistaken for a real declaration.
 * Removes CSS/JS block comments, HTML/Svelte comments, and `//` line comments
 * (but not the `//` inside URLs like https://).
 */
function stripComments(content: string): string {
  return content
    .replace(/\/\*[\s\S]*?\*\//g, '') // /* ... */
    .replace(/<!--[\s\S]*?-->/g, '') // <!-- ... -->
    .replace(/(^|[^:])\/\/[^\n]*/g, '$1') // // line comment, not ://
}

const readClean = (p: string) => stripComments(read(p))

/** The token file is the single source of truth and is exempt from literal bans. */
const isTokensFile = (p: string) => p === TOKENS_FILE

const svelteAndCssFiles = walk(SRC_DIR, ['.svelte', '.css'])
const svelteFiles = walk(SRC_DIR, ['.svelte'])

// Sanity: the walk must actually find the source we intend to audit. A silent
// empty list would make every assertion vacuously pass.
describe('design-system audit / fixture sanity', () => {
  it('discovers styled source files to audit', () => {
    expect(svelteAndCssFiles.length).toBeGreaterThan(0)
    expect(svelteFiles.length).toBeGreaterThan(0)
  })
})

// ---------------------------------------------------------------------------
// Requirement 11.1 — every visual surface renders with border-radius: 0px,
// with no non-zero exceptions.
// ---------------------------------------------------------------------------
describe('Req 11.1 — sharp corners only (border-radius is always 0)', () => {
  // A border-radius value is acceptable when every component is zero or a
  // reference to a radius token that is itself defined as 0.
  const ZERO_PART = /^(0(px)?|var\(--radius(-none)?\))$/

  function nonZeroRadiusDeclarations(content: string): string[] {
    const offenders: string[] = []
    const re = /border-radius\s*:\s*([^;}{]+)/gi
    let m: RegExpExecArray | null
    while ((m = re.exec(content)) !== null) {
      const value = m[1].replace(/!important/gi, '').trim()
      const parts = value.split(/[\s,/]+/).filter(Boolean)
      const allZero = parts.length > 0 && parts.every((p) => ZERO_PART.test(p))
      if (!allZero) offenders.push(value)
    }
    return offenders
  }

  it('declares no non-zero border-radius anywhere under src/', () => {
    const violations: string[] = []
    for (const file of svelteAndCssFiles) {
      for (const value of nonZeroRadiusDeclarations(readClean(file))) {
        violations.push(`${rel(file)}: border-radius: ${value}`)
      }
    }
    expect(violations, `non-zero border-radius found:\n${violations.join('\n')}`).toEqual([])
  })

  it('enforces the global square-corner rule in the token layer', () => {
    const tokens = read(TOKENS_FILE)
    // The global reset must pin every element (and pseudo-elements) to 0.
    expect(tokens).toMatch(/border-radius\s*:\s*0\s*!important/i)
    // The radius tokens themselves must be 0.
    expect(tokens).toMatch(/--radius\s*:\s*0\s*;/)
    expect(tokens).toMatch(/--radius-none\s*:\s*0\s*;/)
  })
})

// ---------------------------------------------------------------------------
// Requirement 11.2 — all padding/margin/gap values are drawn from the single
// spacing scale (var(--space-*)) or are 0. Hardcoded non-zero px literals that
// do not reference a spacing token are forbidden in component styles.
// ---------------------------------------------------------------------------
describe('Req 11.2 — spacing comes from the scale, not hardcoded px', () => {
  // Match spacing-related declarations and inspect their value.
  const SPACING_DECL =
    /(padding|margin|gap|row-gap|column-gap)(-(?:top|right|bottom|left))?\s*:\s*([^;{}]+)/gi
  const NONZERO_PX = /(?<![\w.])(\d*\.?\d+)px/g

  function hardcodedSpacing(content: string): string[] {
    const offenders: string[] = []
    let m: RegExpExecArray | null
    while ((m = SPACING_DECL.exec(content)) !== null) {
      const prop = m[1] + (m[2] ?? '')
      const value = m[3].trim()
      // A declaration that draws from the spacing scale is compliant even if it
      // also contains a deliberate hairline literal alongside the token.
      if (value.includes('var(--space')) continue
      let px: RegExpExecArray | null
      let hasNonZeroPx = false
      NONZERO_PX.lastIndex = 0
      while ((px = NONZERO_PX.exec(value)) !== null) {
        if (parseFloat(px[1]) !== 0) hasNonZeroPx = true
      }
      if (hasNonZeroPx) offenders.push(`${prop}: ${value}`)
    }
    return offenders
  }

  it('uses spacing tokens (or 0) for all padding/margin/gap in components', () => {
    const violations: string[] = []
    for (const file of svelteFiles) {
      for (const decl of hardcodedSpacing(readClean(file))) {
        violations.push(`${rel(file)}: ${decl}`)
      }
    }
    expect(
      violations,
      `hardcoded non-token spacing found:\n${violations.join('\n')}`,
    ).toEqual([])
  })
})

// ---------------------------------------------------------------------------
// Requirement 11.3 — container-level surfaces use a minimum interior padding of
// 16px, drawn from the scale. The token --container-padding encodes this floor.
// ---------------------------------------------------------------------------
describe('Req 11.3 — container padding is at least 16px', () => {
  /** Parse `--name: value;` custom-property declarations from tokens.css. */
  function tokenMap(content: string): Map<string, string> {
    const map = new Map<string, string>()
    const re = /(--[\w-]+)\s*:\s*([^;]+);/g
    let m: RegExpExecArray | null
    while ((m = re.exec(content)) !== null) {
      // Keep the first (:root) definition for each token.
      if (!map.has(m[1])) map.set(m[1], m[2].trim())
    }
    return map
  }

  /** Resolve a token value to a px number, following var() references. */
  function resolvePx(value: string, map: Map<string, string>, depth = 0): number | null {
    if (depth > 10) return null
    const varMatch = value.match(/^var\((--[\w-]+)\)$/)
    if (varMatch) {
      const ref = map.get(varMatch[1])
      return ref ? resolvePx(ref, map, depth + 1) : null
    }
    const pxMatch = value.match(/^(\d*\.?\d+)px$/)
    if (pxMatch) return parseFloat(pxMatch[1])
    const zero = value.match(/^0$/)
    if (zero) return 0
    return null
  }

  it('resolves --container-padding to >= 16px', () => {
    const map = tokenMap(read(TOKENS_FILE))
    expect(map.has('--container-padding')).toBe(true)
    const px = resolvePx(map.get('--container-padding')!, map)
    expect(px, '--container-padding did not resolve to a px value').not.toBeNull()
    expect(px!).toBeGreaterThanOrEqual(16)
  })

  it('defines a 4px-based spacing scale whose step 4 equals 16px', () => {
    const map = tokenMap(read(TOKENS_FILE))
    // The scale is integer multiples of a 4px base.
    expect(resolvePx(map.get('--space-base') ?? '', map)).toBe(4)
    expect(resolvePx(map.get('--space-4') ?? '', map)).toBe(16)
  })
})

// ---------------------------------------------------------------------------
// Requirement 11.4 — a single token-based design system; no hardcoded color or
// typography literals outside the token file.
// ---------------------------------------------------------------------------
describe('Req 11.4 — colors come from tokens, not hardcoded literals', () => {
  const HEX = /#[0-9a-fA-F]{3,8}\b/g

  // Allowlist: pure-black overlays/shadows are scrims and elevation effects,
  // not palette colors. They are documented exceptions used for modal/drawer
  // backdrops and box-shadows (e.g. App.svelte sidebar drawer + CommandPalette
  // overlay). Any other rgb()/rgba() literal would be a hardcoded palette color
  // and is reported as a violation.
  const ALLOWED_RGBA = /rgba?\(\s*0\s*,\s*0\s*,\s*0\s*,\s*(?:0|1|0?\.\d+)\s*\)/i
  const ANY_RGB = /rgba?\([^)]*\)/gi

  it('declares no hardcoded hex colors outside the token file', () => {
    const violations: string[] = []
    for (const file of svelteAndCssFiles) {
      if (isTokensFile(file)) continue
      const matches = readClean(file).match(HEX)
      if (matches) violations.push(`${rel(file)}: ${matches.join(', ')}`)
    }
    expect(
      violations,
      `hardcoded hex colors found outside tokens.css:\n${violations.join('\n')}`,
    ).toEqual([])
  })

  it('declares no hardcoded rgb()/rgba() palette colors outside the token file', () => {
    const violations: string[] = []
    for (const file of svelteAndCssFiles) {
      if (isTokensFile(file)) continue
      const content = readClean(file)
      let m: RegExpExecArray | null
      ANY_RGB.lastIndex = 0
      while ((m = ANY_RGB.exec(content)) !== null) {
        const literal = m[0]
        // Allow documented black scrims/shadows only.
        if (ALLOWED_RGBA.test(literal)) continue
        violations.push(`${rel(file)}: ${literal}`)
      }
    }
    expect(
      violations,
      `hardcoded non-allowlisted rgb/rgba colors found:\n${violations.join('\n')}`,
    ).toEqual([])
  })

  it('confirms the color palette lives in tokens.css', () => {
    // The hex palette must exist somewhere — and that somewhere is the token file.
    expect(read(TOKENS_FILE).match(HEX)?.length ?? 0).toBeGreaterThan(0)
  })
})
