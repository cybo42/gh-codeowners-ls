// Package output renders a Report as human-readable text or JSON.
package output

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/cli/go-gh/v2/pkg/tableprinter"
	"github.com/hmarr/codeowners"

	"github.com/cybo42/gh-codeowners-ls/internal/diff"
	"github.com/cybo42/gh-codeowners-ls/internal/owners"
)

// HumanOptions configures human-readable rendering.
type HumanOptions struct {
	IsTTY     bool
	Width     int
	ShowGroup bool // print the owner summary only
	ShowFiles bool // print the per-file table only
	ShowRules bool // show the matching CODEOWNERS pattern + line number
}

// Human writes report as the summary-then-detail human output. When neither
// ShowGroup nor ShowFiles is set, both sections are printed.
func Human(w io.Writer, report owners.Report, opts HumanOptions) error {
	showGroup := opts.ShowGroup || !opts.ShowFiles
	showFiles := opts.ShowFiles || !opts.ShowGroup

	fmt.Fprintln(w, headerLine(report))
	fmt.Fprintln(w)

	if showGroup {
		renderGroups(w, report)
	}
	if showGroup && showFiles {
		fmt.Fprintln(w)
	}
	if showFiles {
		if err := renderFiles(w, report, opts); err != nil {
			return err
		}
	}
	return nil
}

func headerLine(report owners.Report) string {
	fileWord := "files"
	if len(report.Files) == 1 {
		fileWord = "file"
	}

	codeownersPart := "no CODEOWNERS file found"
	if report.CodeownersPath != "" {
		codeownersPart = fmt.Sprintf("CODEOWNERS @ %s", report.CodeownersPath)
	}

	line := fmt.Sprintf("%s  •  %d %s  •  %s", report.Source.Label, len(report.Files), fileWord, codeownersPart)
	if repoPart := repoDisplay(report.Repo); repoPart != "" {
		line = repoPart + "  •  " + line
	}
	if report.Source.Base != "" {
		line += fmt.Sprintf(" (base: %s)", shortRef(report.Source.Base))
	}
	if report.Source.Truncated {
		line += "\n⚠ file list truncated at GitHub's API ceiling — some files may be missing"
	}
	if report.Identity.Warning != "" {
		line += fmt.Sprintf("\n⚠ %s", report.Identity.Warning)
	}
	return line
}

func renderGroups(w io.Writer, report owners.Report) {
	if len(report.ByOwner) == 0 {
		fmt.Fprintln(w, "(no changed files)")
		return
	}
	for _, g := range report.ByOwner {
		mine := ""
		if g.Mine {
			mine = "  ← you"
		}
		word := "files"
		if len(g.Files) == 1 {
			word = "file"
		}
		fmt.Fprintf(w, "%-16s %d %s%s\n", g.Owner, len(g.Files), word, mine)
	}
}

func renderFiles(w io.Writer, report owners.Report, opts HumanOptions) error {
	tp := tableprinter.New(w, opts.IsTTY, opts.Width)
	tp.AddHeader(headerColumns(opts))

	for _, fo := range report.Files {
		writeRow(tp, fo, opts)
	}
	return tp.Render()
}

func headerColumns(opts HumanOptions) []string {
	cols := []string{"STATUS", "FILE", "OWNERS"}
	if opts.ShowRules {
		cols = append(cols, "RULE", "VIA")
	}
	return append(cols, "MINE")
}

func writeRow(tp tableprinter.TablePrinter, fo owners.FileOwnership, opts HumanOptions) {
	tp.AddField(string(fo.File.Status))
	tp.AddField(filePathDisplay(fo.File))
	tp.AddField(ownersDisplay(fo.Owners))
	if opts.ShowRules {
		tp.AddField(ruleDisplay(fo.Rule))
		tp.AddField(fo.MineVia)
	}
	mine := ""
	if fo.Mine {
		mine = "✓"
	}
	tp.AddField(mine)
	tp.EndRow()
}

func filePathDisplay(f diff.ChangedFile) string {
	if f.PreviousPath != "" {
		return fmt.Sprintf("%s → %s", f.PreviousPath, f.Path)
	}
	return f.Path
}

func ownersDisplay(owns []codeowners.Owner) string {
	if len(owns) == 0 {
		return "(unowned)"
	}
	parts := make([]string, len(owns))
	for i, o := range owns {
		parts[i] = o.String()
	}
	return strings.Join(parts, ", ")
}

func ruleDisplay(r *codeowners.Rule) string {
	if r == nil {
		return "-"
	}
	return fmt.Sprintf("%s (L%d)", r.RawPattern(), r.LineNumber)
}

func repoDisplay(repo repository.Repository) string {
	if repo.Owner == "" || repo.Name == "" {
		return ""
	}
	if repo.Host != "" && repo.Host != "github.com" {
		return fmt.Sprintf("%s/%s/%s", repo.Host, repo.Owner, repo.Name)
	}
	return fmt.Sprintf("%s/%s", repo.Owner, repo.Name)
}

var fullSHARegex = regexp.MustCompile(`^[0-9a-f]{40}$`)

// shortRef abbreviates a full SHA for display; branch/tag names pass through
// unchanged.
func shortRef(ref string) string {
	if fullSHARegex.MatchString(ref) {
		return ref[:8]
	}
	return ref
}
