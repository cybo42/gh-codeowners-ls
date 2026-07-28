package output

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hmarr/codeowners"

	"github.com/cybo42/gh-codeowners-ls/internal/diff"
	"github.com/cybo42/gh-codeowners-ls/internal/owners"
)

var update = flag.Bool("update", false, "update golden files")

const sampleCodeowners = `*                    @acme/default
internal/billing/    @acme/platform
web/                 @acme/web
`

func sampleReport(t *testing.T) owners.Report {
	t.Helper()

	ruleset, err := codeowners.ParseFile(strings.NewReader(sampleCodeowners))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	changed := []diff.ChangedFile{
		{Path: "internal/billing/webhook.go", Status: diff.StatusModified},
		{Path: "internal/billing/retry.go", Status: diff.StatusAdded},
		{Path: "web/src/Checkout.tsx", Status: diff.StatusModified},
		{Path: "scripts/tmp.sh", Status: diff.StatusModified},
	}

	identity := owners.Identity{
		Login:      "alice",
		TeamSlugs:  map[string]bool{"acme/platform": true},
		Emails:     map[string]bool{},
		TeamsKnown: true,
	}

	files := owners.Resolve(ruleset, changed, identity)

	return owners.Report{
		Source: diff.Source{
			Kind:  diff.SourcePR,
			PR:    482,
			Base:  "main",
			Head:  "feature/billing",
			Label: `PR #482 "Add billing webhooks"`,
		},
		Identity:       identity,
		CodeownersPath: ".github/CODEOWNERS",
		Files:          files,
		ByOwner:        owners.Group(files),
		Unowned:        owners.Unowned(files),
	}
}

func compareGolden(t *testing.T, goldenName string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", goldenName)

	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output does not match %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

func TestHuman_Default(t *testing.T) {
	report := sampleReport(t)
	var buf bytes.Buffer
	if err := Human(&buf, report, HumanOptions{IsTTY: false, Width: 80}); err != nil {
		t.Fatalf("Human: %v", err)
	}
	compareGolden(t, "human_default.golden", buf.Bytes())
}

func TestHuman_Rules(t *testing.T) {
	report := sampleReport(t)
	var buf bytes.Buffer
	opts := HumanOptions{IsTTY: false, Width: 80, ShowFiles: true, ShowRules: true}
	if err := Human(&buf, report, opts); err != nil {
		t.Fatalf("Human: %v", err)
	}
	compareGolden(t, "human_rules.golden", buf.Bytes())
}

func TestHuman_GroupOnly(t *testing.T) {
	report := sampleReport(t)
	var buf bytes.Buffer
	if err := Human(&buf, report, HumanOptions{IsTTY: false, Width: 80, ShowGroup: true}); err != nil {
		t.Fatalf("Human: %v", err)
	}
	compareGolden(t, "human_group.golden", buf.Bytes())
}

func TestJSON(t *testing.T) {
	report := sampleReport(t)
	var buf bytes.Buffer
	if err := JSON(&buf, report); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	compareGolden(t, "report.golden.json", buf.Bytes())
}
