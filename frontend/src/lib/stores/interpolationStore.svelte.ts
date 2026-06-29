// interpolationStore provides the data the editor needs to flag unresolved
// {{tokens}} inline (Requirement 6.6). It is a pure, client-side mirror of the
// Go core's httpcore.InterpolateString token logic: tokens are `{{name}}`,
// names may be any run of characters except braces, and matching against the
// active environment's Variable names is case-sensitive. It performs no I/O —
// it reads the active environment from environmentsStore.

import { environmentsStore } from './environmentsStore.svelte'

// Mirrors httpcore.tokenPattern: `\{\{([^{}]*)\}\}`. The capture group is the
// raw variable name between the braces. The global flag lets us walk every
// token in a string.
const TOKEN_PATTERN = /\{\{([^{}]*)\}\}/g

class InterpolationStore {
  /**
   * The set of Variable names defined in the active environment, used to decide
   * whether a token resolves. Derived from environmentsStore, so it updates
   * whenever the active environment or its variables change. When no
   * environment is active the set is empty and every token is unresolved.
   */
  definedNames = $derived.by<Set<string>>(() => {
    const env = environmentsStore.activeEnvironment
    const names = new Set<string>()
    if (env != null) {
      for (const v of env.variables) {
        // First definition wins, matching the core's lookup; Set.add is
        // naturally idempotent so duplicates collapse to a single name.
        names.add(v.name)
      }
    }
    return names
  })

  /**
   * Return the distinct unresolved tokens in `input`, in order of first
   * appearance, each as its full `{{name}}` text. A token is unresolved when
   * its name has no case-sensitive match among the active environment's
   * variables (including when no environment is active). Mirrors the unresolved
   * list produced by httpcore.InterpolateString.
   */
  findUnresolved(input: string): string[] {
    if (input === '') return []
    const defined = this.definedNames
    const seen = new Set<string>()
    const unresolved: string[] = []
    for (const match of input.matchAll(TOKEN_PATTERN)) {
      const token = match[0]
      const name = match[1]
      if (!defined.has(name) && !seen.has(token)) {
        seen.add(token)
        unresolved.push(token)
      }
    }
    return unresolved
  }

  /** True when `input` contains at least one unresolved {{token}}. */
  hasUnresolved(input: string): boolean {
    if (input === '') return false
    const defined = this.definedNames
    for (const match of input.matchAll(TOKEN_PATTERN)) {
      if (!defined.has(match[1])) return true
    }
    return false
  }
}

export const interpolationStore = new InterpolationStore()
