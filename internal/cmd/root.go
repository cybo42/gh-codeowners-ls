// Package cmd wires the gh-codeowners-ls CLI: flags, orchestration of the
// diff/owners/output packages, and exit codes.
package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/hmarr/codeowners"
	"github.com/spf13/cobra"

	"github.com/cybo42/gh-codeowners-ls/internal/diff"
	"github.com/cybo42/gh-codeowners-ls/internal/output"
	"github.com/cybo42/gh-codeowners-ls/internal/owners"
)

// Exit codes, per the CLI contract: filters never change the exit code on
// their own — CI gating is opt-in via --fail-on-unowned/--fail-on-empty — so
// the tool stays safe to pipe.
const (
	exitOK           = 0
	exitRuntimeError = 1
	exitUsageError   = 2
	exitUnowned      = 3
	exitEmpty        = 4
)

type options struct {
	pr                 string
	base               string
	head               string
	staged             bool
	includeUncommitted bool

	mine    bool
	unowned bool
	owner   []string

	group   bool
	files   bool
	rules   bool
	jsonOut bool
	noColor bool

	validate      bool
	failOnUnowned bool
	failOnEmpty   bool

	repo       string
	codeowners string
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	exitCode := exitOK
	root := newRootCmd(&exitCode)
	root.SilenceUsage = true
	root.SilenceErrors = true
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return exitUsageError
	}
	return exitCode
}

func newRootCmd(exitCode *int) *cobra.Command {
	var o options

	cmd := &cobra.Command{
		Use:   "codeowners-ls [<base>...<head>]",
		Short: "Map a diff's changed files to their CODEOWNERS",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			var rangeArg string
			if len(args) == 1 {
				rangeArg = args[0]
			}
			if err := run(c, o, rangeArg, exitCode); err != nil {
				fmt.Fprintln(c.ErrOrStderr(), "Error:", err)
				*exitCode = exitRuntimeError
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&o.pr, "pr", "", "use a pull request's file list (number or URL; a URL also selects the repository)")
	flags.StringVar(&o.base, "base", "", "base ref for a local diff (default: merge-base with default branch)")
	flags.StringVar(&o.head, "head", "", "head ref for a local diff (default: HEAD)")
	flags.BoolVar(&o.staged, "staged", false, "include only staged changes")
	flags.BoolVar(&o.includeUncommitted, "include-uncommitted", true, "include working-tree changes in a local diff")

	flags.BoolVar(&o.mine, "mine", false, "only show files owned by you (or your teams)")
	flags.BoolVar(&o.unowned, "unowned", false, "only show files with no owner")
	flags.StringArrayVar(&o.owner, "owner", nil, "only show files owned by the given user/team (repeatable)")

	flags.BoolVar(&o.group, "group", false, "print the owner summary only")
	flags.BoolVar(&o.files, "files", false, "print the per-file table only")
	flags.BoolVar(&o.rules, "rules", false, "show the matching CODEOWNERS pattern and line number")
	flags.BoolVar(&o.jsonOut, "json", false, "machine-readable output")
	flags.BoolVar(&o.noColor, "no-color", false, "disable color")

	flags.BoolVar(&o.validate, "validate", false, "report CODEOWNERS syntax/permission errors from GitHub")
	flags.BoolVar(&o.failOnUnowned, "fail-on-unowned", false, "exit 3 if any changed file has no owner")
	flags.BoolVar(&o.failOnEmpty, "fail-on-empty", false, "exit 4 if a filter (e.g. --mine) matches nothing")

	flags.StringVar(&o.repo, "repo", "", "target repository ([HOST/]OWNER/REPO)")
	flags.StringVar(&o.codeowners, "codeowners", "", "explicit CODEOWNERS file path")

	return cmd
}

func run(cmd *cobra.Command, o options, rangeArg string, exitCode *int) error {
	var prRef diff.PRRef
	var err error
	if o.pr != "" {
		prRef, err = diff.ParsePRRef(o.pr)
		if err != nil {
			return err
		}
	}

	repo, repoSpec, err := resolveRepo(o.repo, prRef)
	if err != nil {
		return err
	}

	// The diff/CODEOWNERS/validate client stays uncached: a reviewer who
	// fixes a CODEOWNERS typo or pushes a new commit expects the next run to
	// see it immediately. Only identity (login/teams/emails) is cached below
	// — those change rarely within a review session.
	client, err := api.NewRESTClient(api.ClientOptions{Host: repo.Host})
	if err != nil {
		return fmt.Errorf("build API client: %w", err)
	}

	src, err := diff.Resolve(diff.ResolveOptions{
		Repo:               repo,
		Client:             client,
		PRNumber:           prRef.Number,
		RepoSpec:           repoSpec,
		Base:               o.base,
		Head:               o.head,
		Range:              rangeArg,
		Staged:             o.staged,
		IncludeUncommitted: o.includeUncommitted,
	})
	if err != nil {
		return err
	}

	baseRefForCodeowners := ""
	if src.Kind == diff.SourcePR {
		baseRefForCodeowners = src.Base
	}
	ruleset, codeownersPath, err := owners.LoadRuleset(owners.LoadOptions{
		ExplicitPath: o.codeowners,
		Client:       client,
		Repo:         repo,
		BaseRef:      baseRefForCodeowners,
	})
	if err != nil {
		return err
	}

	identityClient, err := api.NewRESTClient(api.ClientOptions{Host: repo.Host, EnableCache: true, CacheTTL: 3 * time.Minute})
	if err != nil {
		return fmt.Errorf("build API client: %w", err)
	}
	identity, err := owners.ResolveIdentity(identityClient, repo)
	if err != nil {
		return err
	}

	files := owners.Resolve(ruleset, src.Files, identity)

	var validationErrs []owners.ValidationError
	if o.validate {
		ref := src.Base
		if ref == "" {
			ref = "HEAD"
		}
		validationErrs, err = owners.Validate(client, repo, ref)
		if err != nil {
			return err
		}
	}

	filtered := applyFilters(files, o)

	report := owners.Report{
		Repo:           repo,
		Source:         src,
		Identity:       identity,
		CodeownersPath: codeownersPath,
		Files:          filtered,
		ByOwner:        owners.Group(filtered),
		Unowned:        owners.Unowned(filtered),
		Validation:     validationErrs,
	}

	if err := render(cmd, report, o); err != nil {
		return err
	}

	if o.failOnEmpty && len(filtered) == 0 {
		*exitCode = exitEmpty
	}
	if o.failOnUnowned && len(report.Unowned) > 0 {
		*exitCode = exitUnowned
	}
	return nil
}

// resolveRepo determines the target repository and, when it was explicitly
// targeted (via --repo or a --pr URL), a "HOST/OWNER/NAME" spec to forward to
// gh subprocesses so they don't fall back to resolving the cwd's repo.
// RepoSpec is deliberately empty when the repo was auto-detected: passing
// --repo unconditionally to `gh pr view` would defeat its own base-repo
// resolution for cross-fork PRs.
func resolveRepo(explicit string, ref diff.PRRef) (repository.Repository, string, error) {
	if explicit != "" {
		repo, err := repository.Parse(explicit)
		if err != nil {
			return repository.Repository{}, "", err
		}
		if ref.HasRepo && !sameRepo(repo, ref.Repo) {
			return repository.Repository{}, "", fmt.Errorf(
				"--repo %s conflicts with --pr URL for %s",
				repoSpecString(repo), repoSpecString(ref.Repo),
			)
		}
		return repo, repoSpecString(repo), nil
	}

	if ref.HasRepo {
		return ref.Repo, repoSpecString(ref.Repo), nil
	}

	repo, err := repository.Current()
	if err != nil {
		return repository.Repository{}, "", fmt.Errorf("%w (use --pr <url> or --repo OWNER/REPO to target a repository without a local clone)", err)
	}
	return repo, "", nil
}

func sameRepo(a, b repository.Repository) bool {
	return strings.EqualFold(a.Host, b.Host) && strings.EqualFold(a.Owner, b.Owner) && strings.EqualFold(a.Name, b.Name)
}

func repoSpecString(r repository.Repository) string {
	return fmt.Sprintf("%s/%s/%s", r.Host, r.Owner, r.Name)
}

func applyFilters(files []owners.FileOwnership, o options) []owners.FileOwnership {
	if !o.mine && !o.unowned && len(o.owner) == 0 {
		return files
	}

	wanted := make(map[string]bool, len(o.owner))
	for _, s := range o.owner {
		wanted[strings.ToLower(strings.TrimPrefix(s, "@"))] = true
	}

	var out []owners.FileOwnership
	for _, fo := range files {
		if o.mine && !fo.Mine {
			continue
		}
		if o.unowned && len(fo.Owners) != 0 {
			continue
		}
		if len(wanted) > 0 && !matchesOwner(fo.Owners, wanted) {
			continue
		}
		out = append(out, fo)
	}
	return out
}

func matchesOwner(owns []codeowners.Owner, wanted map[string]bool) bool {
	for _, o := range owns {
		if wanted[strings.ToLower(strings.TrimPrefix(o.Value, "@"))] {
			return true
		}
	}
	return false
}

func render(cmd *cobra.Command, report owners.Report, o options) error {
	w := cmd.OutOrStdout()
	if o.jsonOut {
		return output.JSON(w, report)
	}

	t := term.FromEnv()
	width, _, err := t.Size()
	if err != nil || width <= 0 {
		width = 80
	}

	return output.Human(w, report, output.HumanOptions{
		IsTTY:     t.IsTerminalOutput() && !o.noColor,
		Width:     width,
		ShowGroup: o.group,
		ShowFiles: o.files,
		ShowRules: o.rules,
	})
}
