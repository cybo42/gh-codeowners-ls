// Package diff resolves the set of changed files for a review: from a pull
// request, an explicit ref range, or auto-detected from the current branch.
package diff

// Status is a changed file's git status, using single-letter git codes.
type Status string

const (
	StatusAdded    Status = "A"
	StatusModified Status = "M"
	StatusRemoved  Status = "D"
	StatusRenamed  Status = "R"
	StatusCopied   Status = "C"
)

// ChangedFile is one file touched by a diff.
type ChangedFile struct {
	Path         string // current path
	PreviousPath string // set for renames/copies
	Status       Status
	Additions    int // 0 when unavailable (local diff w/o --numstat)
	Deletions    int
}

// SourceKind identifies where a Source's file list came from.
type SourceKind string

const (
	SourcePR    SourceKind = "pr"
	SourceLocal SourceKind = "local"
	SourceRange SourceKind = "range"
)

// Source is the resolved diff: where it came from and which files changed.
type Source struct {
	Kind      SourceKind
	PR        int
	Base      string
	Head      string
	Label     string // human description, e.g. `PR #482 "Add billing webhooks"`
	Files     []ChangedFile
	Truncated bool // hit the API's file-count ceiling
}
// touched
