// Package owners resolves CODEOWNERS ownership for a diff's changed files,
// including who the authenticated gh user is and which files are theirs.
package owners

import (
	"github.com/hmarr/codeowners"

	"github.com/cybo42/gh-codeowners-ls/internal/diff"
)

// Identity is who the authenticated gh user is, for "is this file mine?"
// matching.
type Identity struct {
	Login      string
	TeamSlugs  map[string]bool // "org/team-slug", lowercased
	Emails     map[string]bool // verified emails, lowercased
	TeamsKnown bool            // false when read:org scope was missing
	Warning    string          // why teams/emails are unknown, surfaced to stderr
}

// FileOwnership is one changed file's resolved CODEOWNERS ownership.
type FileOwnership struct {
	File    diff.ChangedFile
	Owners  []codeowners.Owner // empty == unowned
	Rule    *codeowners.Rule   // nil when unowned; carries RawPattern + LineNumber
	Mine    bool
	MineVia string // "@login" or "@org/team" that matched
}

// OwnerGroup buckets FileOwnerships under a single owner (or "(unowned)").
type OwnerGroup struct {
	Owner string
	Mine  bool
	Files []FileOwnership
}

// ValidationError is one entry from GitHub's CODEOWNERS validation API.
type ValidationError struct {
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Kind       string `json:"kind"`
	Message    string `json:"message"`
	Path       string `json:"path"`
	Source     string `json:"source"`
	Suggestion string `json:"suggestion"`
}

// Report is everything needed to render output: the diff, the resolved
// ownership, and where CODEOWNERS came from.
type Report struct {
	Source         diff.Source
	Identity       Identity
	CodeownersPath string // display label for where CODEOWNERS was loaded from
	Files          []FileOwnership
	ByOwner        []OwnerGroup
	Unowned        []FileOwnership
	Validation     []ValidationError
}
