package release

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

// Feature: volt-api-client, Property 24: Semantic version tag matching gates releases
//
// Validates: Requirements 13.6
//
// The release workflow triggers only on semver tags (glob
// `v[0-9]+.[0-9]+.[0-9]+*`); IsReleaseTag is the SemVer-correct encoding of that
// gate. The property under test is two-sided:
//
//   - Any tag generated as a valid Semantic_Version tag (a leading `v` plus
//     MAJOR.MINOR.PATCH, optionally with a `-prerelease` and/or `+build` suffix)
//     MUST be accepted — it would trigger a release.
//   - Any tag generated as clearly non-semver (missing the leading `v`, missing
//     a version component, non-numeric in a numeric field, or empty) MUST be
//     rejected — it would NOT trigger a release.
//
// Both directions are checked across >=100 random inputs via testing/quick.

// validTag is a generated, always-valid semver release tag.
type validTag string

// Generate implements quick.Generator, producing a random but always-valid
// `vMAJOR.MINOR.PATCH[-prerelease][+build]` tag.
func (validTag) Generate(rng *rand.Rand, _ int) reflect.Value {
	tag := "v" +
		genNumericComponent(rng) + "." +
		genNumericComponent(rng) + "." +
		genNumericComponent(rng)

	// Optionally append a pre-release suffix: -<ident>(.<ident>)*
	if rng.Intn(2) == 0 {
		tag += "-" + genDottedIdentifiers(rng, genPrereleaseIdent)
	}
	// Optionally append build metadata: +<ident>(.<ident>)*
	if rng.Intn(2) == 0 {
		tag += "+" + genDottedIdentifiers(rng, genBuildIdent)
	}
	return reflect.ValueOf(validTag(tag))
}

// invalidTag is a generated string that is clearly NOT a semver release tag.
type invalidTag string

// Generate implements quick.Generator, producing a random string drawn from
// several families of clearly-malformed tags. Each family is constructed so the
// result can never coincidentally be a valid semver tag.
func (invalidTag) Generate(rng *rand.Rand, _ int) reflect.Value {
	switch rng.Intn(6) {
	case 0:
		// Empty string.
		return reflect.ValueOf(invalidTag(""))
	case 1:
		// Missing the leading `v`: MAJOR.MINOR.PATCH with no prefix.
		s := genNumericComponent(rng) + "." +
			genNumericComponent(rng) + "." +
			genNumericComponent(rng)
		return reflect.ValueOf(invalidTag(s))
	case 2:
		// Missing a numeric component: only vMAJOR.MINOR (two components).
		s := "v" + genNumericComponent(rng) + "." + genNumericComponent(rng)
		return reflect.ValueOf(invalidTag(s))
	case 3:
		// Non-numeric content in a version field: vMAJOR.<letters>.PATCH.
		s := "v" + genNumericComponent(rng) + "." +
			genAlpha(rng) + "." + genNumericComponent(rng)
		return reflect.ValueOf(invalidTag(s))
	case 4:
		// A bare prefix / non-version word, never containing the version core.
		words := []string{"v", "version", "latest", "release", "vX.Y.Z", "v.", "v1", "v1.2"}
		return reflect.ValueOf(invalidTag(words[rng.Intn(len(words))]))
	default:
		// Wrong prefix character before an otherwise-numeric core.
		s := "x" + genNumericComponent(rng) + "." +
			genNumericComponent(rng) + "." + genNumericComponent(rng)
		return reflect.ValueOf(invalidTag(s))
	}
}

// genNumericComponent returns a numeric version identifier with no leading
// zeros: either "0" or a 1..4 digit number whose first digit is 1..9.
func genNumericComponent(rng *rand.Rand) string {
	if rng.Intn(5) == 0 {
		return "0"
	}
	n := rng.Intn(4) + 1 // 1..4 digits
	var b strings.Builder
	b.WriteByte(byte('1' + rng.Intn(9))) // leading digit 1..9
	for i := 1; i < n; i++ {
		b.WriteByte(byte('0' + rng.Intn(10)))
	}
	return b.String()
}

// genDottedIdentifiers builds a dot-separated list of 1..3 identifiers using the
// supplied per-identifier generator.
func genDottedIdentifiers(rng *rand.Rand, gen func(*rand.Rand) string) string {
	count := rng.Intn(3) + 1
	parts := make([]string, count)
	for i := range parts {
		parts[i] = gen(rng)
	}
	return strings.Join(parts, ".")
}

// genPrereleaseIdent returns a single valid pre-release identifier: either a
// numeric identifier without leading zeros or an alphanumeric identifier.
func genPrereleaseIdent(rng *rand.Rand) string {
	if rng.Intn(2) == 0 {
		return genNumericComponent(rng)
	}
	return genAlphanumericIdent(rng)
}

// genBuildIdent returns a single valid build-metadata identifier: any non-empty
// run of [0-9A-Za-z-] (leading zeros are permitted in build metadata).
func genBuildIdent(rng *rand.Rand) string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-"
	n := rng.Intn(5) + 1
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(alphabet[rng.Intn(len(alphabet))])
	}
	return b.String()
}

// genAlphanumericIdent returns an identifier of [0-9A-Za-z-] that is guaranteed
// to contain at least one non-digit, so it is a valid alphanumeric pre-release
// identifier (and never a bare numeric one).
func genAlphanumericIdent(rng *rand.Rand) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-"
	const all = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-"
	n := rng.Intn(4) + 1
	var b strings.Builder
	// Ensure at least one non-digit character.
	b.WriteByte(letters[rng.Intn(len(letters))])
	for i := 0; i < n; i++ {
		b.WriteByte(all[rng.Intn(len(all))])
	}
	return b.String()
}

// genAlpha returns a short run of letters only (no digits), used to corrupt a
// numeric version field.
func genAlpha(rng *rand.Rand) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	n := rng.Intn(4) + 1
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(letters[rng.Intn(len(letters))])
	}
	return b.String()
}

// TestSemverTagGatesReleases is the property-based test for Property 24.
func TestSemverTagGatesReleases(t *testing.T) {
	// Direction 1: every valid semver tag is accepted (would trigger a release).
	acceptValid := func(v validTag) bool {
		return IsReleaseTag(string(v))
	}
	if err := quick.Check(acceptValid, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 24 failed: a valid semver tag was not accepted: %v", err)
	}

	// Direction 2: every clearly non-semver tag is rejected (no release).
	rejectInvalid := func(iv invalidTag) bool {
		return !IsReleaseTag(string(iv))
	}
	if err := quick.Check(rejectInvalid, &quick.Config{MaxCount: 100}); err != nil {
		t.Fatalf("Property 24 failed: a non-semver tag was accepted: %v", err)
	}
}

// TestIsReleaseTagExamples pins concrete examples on both sides of the gate as a
// fast, readable complement to the property test.
func TestIsReleaseTagExamples(t *testing.T) {
	accepted := []string{
		"v0.0.0",
		"v1.2.3",
		"v10.20.30",
		"v1.2.3-rc.1",
		"v1.2.3-alpha",
		"v1.2.3+build.5",
		"v1.2.3-rc.1+build.5",
	}
	for _, tag := range accepted {
		if !IsReleaseTag(tag) {
			t.Errorf("IsReleaseTag(%q) = false, want true", tag)
		}
	}

	rejected := []string{
		"",
		"v",
		"1.2.3",        // missing leading v
		"v1.2",         // missing patch
		"v1",           // missing minor and patch
		"v1.2.3.4",     // too many components
		"vx.2.3",       // non-numeric major
		"v1.2.x",       // non-numeric patch
		"version1.2.3", // wrong prefix
		"v01.2.3",      // leading zero in major
		"latest",
	}
	for _, tag := range rejected {
		if IsReleaseTag(tag) {
			t.Errorf("IsReleaseTag(%q) = true, want false", tag)
		}
	}
}
