package diff

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

// ResolveOptions selects which diff source to use.
type ResolveOptions struct {
	Repo   repository.Repository
	Client *api.RESTClient

	PRNumber int    // 0 if not given
	RepoSpec string // "HOST/OWNER/NAME"; non-empty only when the repo was explicitly targeted
	Base     string
	Head     string
	Range    string // "<base>...<head>" positional argument

	Staged             bool
	IncludeUncommitted bool
}

// Resolve picks a diff source: an explicit --pr, an explicit --base/--head
// (or base...head range), or auto-detection (open PR for the current branch,
// falling back to a local diff against the default branch's merge-base).
func Resolve(opts ResolveOptions) (Source, error) {
	switch {
	case opts.PRNumber != 0:
		return FilesForPR(opts.Client, opts.Repo, opts.PRNumber)
	case opts.Base != "" || opts.Head != "" || opts.Range != "":
		return resolveExplicitLocal(opts)
	default:
		return resolveAuto(opts)
	}
}

func resolveExplicitLocal(opts ResolveOptions) (Source, error) {
	base, head := opts.Base, opts.Head
	if opts.Range != "" {
		b, h, err := splitRange(opts.Range)
		if err != nil {
			return Source{}, err
		}
		if base == "" {
			base = b
		}
		if head == "" {
			head = h
		}
	}

	headRef := head
	if headRef == "" {
		headRef = "HEAD"
	}

	if base == "" {
		def, err := DefaultBranch(opts.RepoSpec)
		if err != nil {
			return Source{}, fmt.Errorf("resolve base ref: %w", err)
		}
		mb, err := MergeBase(def, headRef)
		if err != nil {
			return Source{}, err
		}
		base = mb
	}

	files, err := LocalFiles(LocalDiffOptions{
		Base:               base,
		Head:               head,
		Staged:             opts.Staged,
		IncludeUncommitted: opts.IncludeUncommitted,
	})
	if err != nil {
		return Source{}, err
	}

	return Source{
		Kind:  SourceRange,
		Base:  base,
		Head:  headRef,
		Label: fmt.Sprintf("local diff %s...%s", base, headRef),
		Files: files,
	}, nil
}

func resolveAuto(opts ResolveOptions) (Source, error) {
	if src, ok, err := detectPR(opts); err != nil {
		return Source{}, err
	} else if ok {
		return src, nil
	}

	def, err := DefaultBranch(opts.RepoSpec)
	if err != nil {
		return Source{}, fmt.Errorf("auto-detect diff source: %w", err)
	}
	mb, err := MergeBase(def, "HEAD")
	if err != nil {
		return Source{}, fmt.Errorf("auto-detect diff source: %w", err)
	}
	files, err := LocalFiles(LocalDiffOptions{
		Base:               mb,
		IncludeUncommitted: true,
	})
	if err != nil {
		return Source{}, err
	}

	return Source{
		Kind:  SourceLocal,
		Base:  mb,
		Head:  "HEAD",
		Label: fmt.Sprintf("local diff vs %s (merge-base %s)", def, shortSHA(mb)),
		Files: files,
	}, nil
}

type prView struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	BaseRefName string `json:"baseRefName"`
}

// detectPR looks for an open PR on the current branch via `gh pr view`. gh
// already solves branch-to-PR mapping, including cross-fork PRs, and exits
// non-zero with "no pull requests found" when there is none — that's reused
// here rather than reimplemented over GraphQL. A non-zero exit (no PR, not a
// repo, offline) is treated as "no PR found", not an error.
func detectPR(opts ResolveOptions) (Source, bool, error) {
	args := []string{"pr", "view", "--json", "number,title,baseRefName"}
	if opts.RepoSpec != "" {
		// `pr view`'s -R/--repo requires an explicit PR/branch argument —
		// without one, gh can't correlate "the current branch" to a repo it
		// wasn't given the local checkout for.
		branch, err := CurrentBranch()
		if err != nil || branch == "" {
			return Source{}, false, nil
		}
		args = append(args, branch, "--repo", opts.RepoSpec)
	}
	stdout, _, err := gh.Exec(args...)
	if err != nil {
		return Source{}, false, nil
	}

	var pv prView
	if err := json.Unmarshal(stdout.Bytes(), &pv); err != nil {
		return Source{}, false, fmt.Errorf("parse gh pr view output: %w", err)
	}

	src, err := FilesForPR(opts.Client, opts.Repo, pv.Number)
	if err != nil {
		return Source{}, false, err
	}
	return src, true, nil
}

func splitRange(r string) (string, string, error) {
	parts := strings.SplitN(r, "...", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid range %q: expected format <base>...<head>", r)
	}
	return parts[0], parts[1], nil
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
