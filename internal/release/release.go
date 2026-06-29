// Package release encodes the rule that gates Volt's release pipeline.
//
// The release workflow (.github/workflows/release.yml) triggers only on pushed
// tags that match the glob `v[0-9]+.[0-9]+.[0-9]+*`. That glob is the CI-level
// expression of a single intent (Requirement 13.6): a release is built and
// published if and only if the pushed tag is a Semantic_Version tag — a leading
// `v` followed by `MAJOR.MINOR.PATCH`, optionally with a pre-release and/or
// build-metadata suffix. Tags that are not valid semver tags must never start a
// release.
//
// IsReleaseTag is the pure, SemVer-correct encoding of that gate, kept separate
// from the workflow YAML so the rule can be reasoned about and property-tested
// in isolation.
package release

import "regexp"

// semverTag matches a Semantic Versioning 2.0.0 version string prefixed with the
// conventional `v` release-tag marker, e.g. v1.2.3, v1.2.3-rc.1, v1.2.3+build.5,
// or v1.2.3-rc.1+build.5.
//
//   - MAJOR, MINOR, and PATCH are numeric identifiers with no leading zeros
//     (0 alone is allowed): 0 | [1-9][0-9]*.
//   - An optional pre-release follows a '-' and is a dot-separated list of
//     identifiers; numeric identifiers carry no leading zeros, alphanumeric
//     identifiers may be any [0-9A-Za-z-] run containing a non-digit.
//   - Optional build metadata follows a '+' and is a dot-separated list of
//     non-empty [0-9A-Za-z-] identifiers (leading zeros permitted here).
//
// This is anchored end-to-end so the whole tag must conform; stray characters,
// missing components, or non-numeric version cores are rejected.
var semverTag = regexp.MustCompile(
	`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)` +
		`(?:-(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)` +
		`(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?` +
		`(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`,
)

// IsReleaseTag reports whether tag is a Semantic_Version release tag and would
// therefore gate (trigger) a release. It returns true exactly for valid
// `v<major>.<minor>.<patch>` tags with optional pre-release/build suffixes, and
// false for any tag that is not a valid semver tag (Requirement 13.6).
func IsReleaseTag(tag string) bool {
	return semverTag.MatchString(tag)
}
