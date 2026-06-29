package httpcore

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"

	"volt/internal/model"
)

// Feature: volt-api-client, Property 8: Variable interpolation resolves defined tokens and passes unresolved tokens through literally

// interpCase is a generated interpolation scenario. It carries both the raw
// input string handed to InterpolateString and the structured segments it was
// assembled from, so the test can compute the expected result from an
// independent oracle rather than re-running the production regex.
type interpCase struct {
	env      model.Environment
	segments []segment
	in       string
}

// segment is one piece of the generated input: either literal text (no braces)
// or a {{name}} interpolation token.
type segment struct {
	isToken bool
	text    string // literal text when !isToken; the token name when isToken
}

// nameRunes is the alphabet used for variable and token names. It deliberately
// mixes upper/lower case (to exercise case-sensitive matching) and a space (to
// exercise the no-whitespace-trimming rule) while excluding braces so a name
// can never split a token.
var nameRunes = []rune("abcXYZ012 ")

// literalRunes is the alphabet for literal segments. It excludes '{' and '}'
// entirely so the only brace sequences in a generated string are real tokens.
var literalRunes = []rune("hello/world.:?=&-_ 123ABC")

func randString(r *rand.Rand, runes []rune, minLen, maxLen int) string {
	n := minLen + r.Intn(maxLen-minLen+1)
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteRune(runes[r.Intn(len(runes))])
	}
	return b.String()
}

// Generate implements quick.Generator, producing a random environment and an
// input string built from a random mix of literal text and {{name}} tokens.
// Some tokens reference defined variables; others reference names that are
// guaranteed to be undefined.
func (interpCase) Generate(r *rand.Rand, size int) reflect.Value {
	// Build a set of defined variable names (case-sensitive, unique, non-empty).
	numVars := r.Intn(5) // 0..4 variables (0 exercises the "no active env vars" path)
	defined := make([]string, 0, numVars)
	seen := map[string]struct{}{}
	vars := make([]model.Variable, 0, numVars)
	for i := 0; i < numVars; i++ {
		name := randString(r, nameRunes, 1, 6)
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		defined = append(defined, name)
		vars = append(vars, model.Variable{Name: name, Value: randString(r, literalRunes, 0, 8)})
	}

	env := model.Environment{ID: "env", Name: "gen", Variables: vars, Active: numVars > 0}

	// Assemble the input from 0..8 segments.
	numSegs := r.Intn(9)
	segs := make([]segment, 0, numSegs)
	var b strings.Builder
	for i := 0; i < numSegs; i++ {
		switch r.Intn(3) {
		case 0: // literal text
			lit := randString(r, literalRunes, 0, 6)
			segs = append(segs, segment{isToken: false, text: lit})
			b.WriteString(lit)
		case 1: // token referencing a defined variable (if any exist)
			var name string
			if len(defined) > 0 {
				name = defined[r.Intn(len(defined))]
			} else {
				name = randString(r, nameRunes, 1, 6)
			}
			segs = append(segs, segment{isToken: true, text: name})
			b.WriteString("{{")
			b.WriteString(name)
			b.WriteString("}}")
		default: // token guaranteed-undefined by prefixing an out-of-alphabet marker
			name := "~undef~" + randString(r, nameRunes, 0, 4)
			segs = append(segs, segment{isToken: true, text: name})
			b.WriteString("{{")
			b.WriteString(name)
			b.WriteString("}}")
		}
	}

	return reflect.ValueOf(interpCase{env: env, segments: segs, in: b.String()})
}

// oracle independently computes the expected output and unresolved-token list
// from the structured segments, mirroring the documented contract: defined
// names (first definition wins, case-sensitive, no trimming) resolve to their
// value; every other token is passed through literally and reported once in
// order of first appearance.
func (c interpCase) oracle() (string, []string) {
	values := map[string]string{}
	for _, v := range c.env.Variables {
		if _, exists := values[v.Name]; !exists {
			values[v.Name] = v.Value
		}
	}

	var out strings.Builder
	var unresolved []string
	seen := map[string]struct{}{}
	for _, s := range c.segments {
		if !s.isToken {
			out.WriteString(s.text)
			continue
		}
		if val, ok := values[s.text]; ok {
			out.WriteString(val)
			continue
		}
		token := "{{" + s.text + "}}"
		out.WriteString(token)
		if _, dup := seen[token]; !dup {
			seen[token] = struct{}{}
			unresolved = append(unresolved, token)
		}
	}
	return out.String(), unresolved
}

func TestProp8_Interpolation(t *testing.T) {
	property := func(c interpCase) bool {
		wantOut, wantUnresolved := c.oracle()
		gotOut, gotUnresolved := InterpolateString(c.in, c.env)

		if gotOut != wantOut {
			t.Logf("input=%q out=%q want=%q", c.in, gotOut, wantOut)
			return false
		}
		if !reflect.DeepEqual(gotUnresolved, wantUnresolved) {
			t.Logf("input=%q unresolved=%#v want=%#v", c.in, gotUnresolved, wantUnresolved)
			return false
		}

		// Every reported unresolved token must still be present verbatim in the
		// output (Requirement 6.8: unresolved tokens are sent through literally).
		for _, tok := range gotUnresolved {
			if !strings.Contains(gotOut, tok) {
				t.Logf("unresolved token %q missing from output %q", tok, gotOut)
				return false
			}
		}
		return true
	}

	if err := quick.Check(property, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
