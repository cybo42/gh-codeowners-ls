package diff

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cli/go-gh/v2"
)

// RepoRoot returns the absolute path to the current git repository's root.
func RepoRoot() (string, error) {
	out, err := runGit("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// MergeBase returns the merge-base commit of base and head.
func MergeBase(base, head string) (string, error) {
	out, err := runGit("merge-base", base, head)
	if err != nil {
		return "", fmt.Errorf("no merge base between %s and %s: %w", base, head, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// DefaultBranch resolves the repository's default branch via `gh repo view`,
// rather than guessing "main".
func DefaultBranch() (string, error) {
	stdout, stderr, err := gh.Exec("repo", "view", "--json", "defaultBranchRef", "--jq", ".defaultBranchRef.name")
	if err != nil {
		return "", fmt.Errorf("resolve default branch: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	branch := strings.TrimSpace(stdout.String())
	if branch == "" {
		return "", fmt.Errorf("could not determine default branch")
	}
	return branch, nil
}

// LocalDiffOptions configures a working-tree/commit diff via `git diff`.
type LocalDiffOptions struct {
	Base   string
	Head   string // empty means diff against the working tree (or HEAD, if Staged)
	Staged bool
	// IncludeUncommitted includes working-tree changes when Head is empty.
	// When false and Head is empty, the diff is Base..HEAD (committed only).
	IncludeUncommitted bool
}

// LocalFiles runs `git diff --name-status -M -z` and parses the result.
// The -z flag is required: without it, git quote-escapes paths containing
// spaces or non-ASCII characters and naive parsing silently corrupts them.
func LocalFiles(opts LocalDiffOptions) ([]ChangedFile, error) {
	args := []string{"diff", "--name-status", "-M", "-z"}
	switch {
	case opts.Staged:
		args = append(args, "--staged", opts.Base)
	case opts.Head != "":
		args = append(args, opts.Base, opts.Head)
	case opts.IncludeUncommitted:
		args = append(args, opts.Base)
	default:
		args = append(args, opts.Base, "HEAD")
	}

	out, err := runGit(args...)
	if err != nil {
		return nil, err
	}
	return parseNameStatusZ(out)
}

func runGit(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// parseNameStatusZ parses NUL-separated `git diff --name-status -z` output.
// Records are "M\0path\0" for simple edits, or "R100\0old\0new\0" for
// renames/copies, where the two paths that follow a R/C status are the
// previous and current path respectively.
func parseNameStatusZ(data []byte) ([]ChangedFile, error) {
	parts := strings.Split(string(data), "\x00")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	var files []ChangedFile
	for i := 0; i < len(parts); {
		statusField := parts[i]
		i++
		if statusField == "" {
			continue
		}

		switch statusField[0] {
		case 'R', 'C':
			if i+1 >= len(parts) {
				return nil, fmt.Errorf("malformed rename/copy record in git diff output")
			}
			oldPath, newPath := parts[i], parts[i+1]
			i += 2
			status := StatusRenamed
			if statusField[0] == 'C' {
				status = StatusCopied
			}
			files = append(files, ChangedFile{Path: newPath, PreviousPath: oldPath, Status: status})
		default:
			if i >= len(parts) {
				return nil, fmt.Errorf("malformed record in git diff output")
			}
			path := parts[i]
			i++
			files = append(files, ChangedFile{Path: path, Status: Status(statusField[:1])})
		}
	}
	return files, nil
}
