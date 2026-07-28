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
	flags.StringVar(&o.pr, "pr", "", "use a pull request's file list (number or URL)")
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
	repo, err := resolveRepo(o.repo)
	if err != nil {
		return err
	}

	client, err := api.NewRESTClient(api.ClientOptions{EnableCache: true, CacheTTL: 3 * time.Minute})
	if err != nil {
		return fmt.Errorf("build API client: %w", err)
	}

	src, err := diff.Resolve(diff.ResolveOptions{
		Repo:               repo,
		Client:             client,
		PR:                 o.pr,
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

	identity, err := owners.ResolveIdentity(client, repo)
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

func resolveRepo(explicit string) (repository.Repository, error) {
	if explicit != "" {
		return repository.Parse(explicit)
	}
	return repository.Current()
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
