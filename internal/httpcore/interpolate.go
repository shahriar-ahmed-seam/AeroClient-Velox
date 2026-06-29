// Package httpcore is the shared, pure request-preparation and execution core
// used by every Volt platform (desktop via Wails, Android via gomobile). It
// performs no persistence and, aside from the isolated network call in
// Execute, no side effects, which makes its rules deterministic and amenable
// to property-based testing.
package httpcore

import (
	"regexp"

	"volt/internal/model"
)

// tokenPattern matches a single {{name}} interpolation token. The captured
// group is the raw variable name between the braces. Names never contain
// braces, so the character class excludes them, keeping the match bounded to a
// single token even when several appear back-to-back.
var tokenPattern = regexp.MustCompile(`\{\{([^{}]*)\}\}`)

// InterpolateString replaces every {{name}} token in in with the value of the
// case-sensitively matching Variable from env, and returns the resolved string
// together with the list of tokens that could not be resolved.
//
// A token whose name has no case-sensitive match in env (including the case
// where env carries no variables, e.g. when no Environment is active) is left
// in the output exactly as written and reported in unresolved. Each distinct
// unresolved token is reported once, in order of first appearance. unresolved
// is nil when every token resolves (or when there are no tokens), so callers
// can treat a nil/empty slice as "fully resolved".
//
// This satisfies Requirements 6.4 (case-sensitive substitution from the active
// environment), 6.5 (the caller applies it to URL, params, headers, and body),
// and 6.8 (unresolved tokens are passed through literally so the request can
// still execute).
func InterpolateString(in string, env model.Environment) (out string, unresolved []string) {
	if in == "" {
		return "", nil
	}

	// Build a case-sensitive lookup of variable name -> value. On duplicate
	// names (which the store layer prevents) the first definition wins.
	values := make(map[string]string, len(env.Variables))
	for _, v := range env.Variables {
		if _, exists := values[v.Name]; !exists {
			values[v.Name] = v.Value
		}
	}

	seenUnresolved := make(map[string]struct{})

	out = tokenPattern.ReplaceAllStringFunc(in, func(token string) string {
		name := tokenPattern.FindStringSubmatch(token)[1]
		if value, ok := values[name]; ok {
			return value
		}
		// Unresolved: keep the original token text verbatim and record it once.
		if _, recorded := seenUnresolved[token]; !recorded {
			seenUnresolved[token] = struct{}{}
			unresolved = append(unresolved, token)
		}
		return token
	})

	return out, unresolved
}
