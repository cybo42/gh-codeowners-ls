package owners

import (
	"testing"

	"github.com/hmarr/codeowners"

	"github.com/cybo42/gh-codeowners-ls/internal/diff"
)

func loadTestRuleset(t *testing.T, name string) codeowners.Ruleset {
	t.Helper()
	rs, err := codeowners.LoadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("LoadFile(%s): %v", name, err)
	}
	return rs
}

func ownerStrings(owners []codeowners.Owner) []string {
	out := make([]string, len(owners))
	for i, o := range owners {
		out[i] = o.String()
	}
	return out
}

func TestResolveFile_LastMatchWinsAndPatterns(t *testing.T) {
	ruleset := loadTestRuleset(t, "CODEOWNERS.basic")
	identity := Identity{TeamSlugs: map[string]bool{}, Emails: map[string]bool{}}

	tests := []struct {
		name       string
		path       string
		wantOwners []string
	}{
		{"unmatched file falls to wildcard default", "random/file.txt", []string{"@acme/default"}},
		{"last-match-wins overrides default", "docs/readme.md", []string{"@acme/docs"}},
		{"directory pattern with multiple owners", "internal/billing/webhook.go", []string{"@acme/platform", "@acme/finance"}},
		{"** recursion", "src/foo/bar.go", []string{"@acme/go-developers"}},
		{"explicit no-owner rule overrides default", "scripts/tmp/file.txt", nil},
		{"email owner", "notice.legal.md", []string{"legal@acme.com"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fo := resolveFile(ruleset, diff.ChangedFile{Path: tc.path}, identity)
			got := ownerStrings(fo.Owners)
			if len(got) != len(tc.wantOwners) {
				t.Fatalf("path %s: owners = %v, want %v", tc.path, got, tc.wantOwners)
			}
			for i, w := range tc.wantOwners {
				if got[i] != w {
					t.Errorf("path %s: owners = %v, want %v", tc.path, got, tc.wantOwners)
					break
				}
			}
		})
	}
}

func TestResolveFile_Rename_UnionsOwnersAcrossBoundary(t *testing.T) {
	ruleset := loadTestRuleset(t, "CODEOWNERS.basic")
	identity := Identity{TeamSlugs: map[string]bool{}, Emails: map[string]bool{}}

	fo := resolveFile(ruleset, diff.ChangedFile{
		Path:         "docs/readme.md",
		PreviousPath: "random/old-readme.md",
	}, identity)

	got := ownerStrings(fo.Owners)
	want := map[string]bool{"@acme/docs": true, "@acme/default": true}
	if len(got) != len(want) {
		t.Fatalf("owners = %v, want union of %v", got, want)
	}
	for _, o := range got {
		if !want[o] {
			t.Errorf("unexpected owner %s in %v", o, got)
		}
	}
}

func TestWhoseMine(t *testing.T) {
	identity := Identity{
		Login:     "alice",
		TeamSlugs: map[string]bool{"acme/platform": true},
		Emails:    map[string]bool{"legal@acme.com": true},
	}

	tests := []struct {
		name    string
		owners  []codeowners.Owner
		wantOk  bool
		wantVia string
	}{
		{
			name:    "username match is case-insensitive",
			owners:  []codeowners.Owner{{Value: "Alice", Type: codeowners.UsernameOwner}},
			wantOk:  true,
			wantVia: "@Alice",
		},
		{
			name:    "team slug match",
			owners:  []codeowners.Owner{{Value: "acme/platform", Type: codeowners.TeamOwner}},
			wantOk:  true,
			wantVia: "@acme/platform",
		},
		{
			name:   "team match requires slug, not display name",
			owners: []codeowners.Owner{{Value: "acme/Platform Team", Type: codeowners.TeamOwner}},
			wantOk: false,
		},
		{
			name:    "email match",
			owners:  []codeowners.Owner{{Value: "legal@acme.com", Type: codeowners.EmailOwner}},
			wantOk:  true,
			wantVia: "legal@acme.com",
		},
		{
			name:   "no match",
			owners: []codeowners.Owner{{Value: "acme/other-team", Type: codeowners.TeamOwner}},
			wantOk: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok, via := whoseMine(tc.owners, identity)
			if ok != tc.wantOk {
				t.Fatalf("whoseMine() ok = %v, want %v", ok, tc.wantOk)
			}
			if ok && via != tc.wantVia {
				t.Errorf("whoseMine() via = %q, want %q", via, tc.wantVia)
			}
		})
	}
}

func TestGroup_SortsMineFirstThenCountThenName(t *testing.T) {
	files := []FileOwnership{
		{Owners: []codeowners.Owner{{Value: "acme/small", Type: codeowners.TeamOwner}}},
		{Owners: []codeowners.Owner{{Value: "acme/big", Type: codeowners.TeamOwner}}},
		{Owners: []codeowners.Owner{{Value: "acme/big", Type: codeowners.TeamOwner}}},
		{Owners: nil},
		{Owners: []codeowners.Owner{{Value: "acme/mine", Type: codeowners.TeamOwner}}, Mine: true},
	}

	groups := Group(files)

	if len(groups) != 4 {
		t.Fatalf("Group() returned %d groups, want 4", len(groups))
	}
	if groups[0].Owner != "@acme/mine" || !groups[0].Mine {
		t.Errorf("group[0] = %+v, want mine group first", groups[0])
	}
	if groups[1].Owner != "@acme/big" {
		t.Errorf("group[1] = %+v, want @acme/big (2 files)", groups[1])
	}
	if groups[2].Owner != "@acme/small" {
		t.Errorf("group[2] = %+v, want @acme/small", groups[2])
	}
	if groups[3].Owner != "(unowned)" {
		t.Errorf("group[3] = %+v, want (unowned) last", groups[3])
	}
}

func TestUnowned(t *testing.T) {
	files := []FileOwnership{
		{File: diff.ChangedFile{Path: "a"}, Owners: []codeowners.Owner{{Value: "acme/x", Type: codeowners.TeamOwner}}},
		{File: diff.ChangedFile{Path: "b"}, Owners: nil},
		{File: diff.ChangedFile{Path: "c"}, Owners: nil},
	}

	unowned := Unowned(files)
	if len(unowned) != 2 {
		t.Fatalf("Unowned() = %d files, want 2", len(unowned))
	}
	if unowned[0].File.Path != "b" || unowned[1].File.Path != "c" {
		t.Errorf("Unowned() = %v, want [b, c]", unowned)
	}
}
