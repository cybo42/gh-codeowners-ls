package owners

import (
	"sort"
	"strings"

	"github.com/hmarr/codeowners"

	"github.com/cybo42/gh-codeowners-ls/internal/diff"
)

// Resolve computes ownership for each changed file against ruleset, marking
// which files belong to identity.
func Resolve(ruleset codeowners.Ruleset, files []diff.ChangedFile, identity Identity) []FileOwnership {
	result := make([]FileOwnership, 0, len(files))
	for _, f := range files {
		result = append(result, resolveFile(ruleset, f, identity))
	}
	return result
}

func resolveFile(ruleset codeowners.Ruleset, f diff.ChangedFile, identity Identity) FileOwnership {
	rule, _ := ruleset.Match(f.Path)

	// A rename can move a file between owners: union owners from both paths,
	// but keep the current path's rule for --rules display since that's the
	// rule that applies going forward.
	if f.PreviousPath != "" {
		prevRule, _ := ruleset.Match(f.PreviousPath)
		switch {
		case rule == nil:
			rule = prevRule
		case prevRule != nil:
			rule = unionRule(rule, prevRule)
		}
	}

	fo := FileOwnership{File: f}
	if rule != nil {
		fo.Owners = rule.Owners
		fo.Rule = rule
	}
	fo.Mine, fo.MineVia = whoseMine(fo.Owners, identity)
	return fo
}

// unionRule merges owners from two rules (used for renames that cross
// CODEOWNERS boundaries). The primary rule's pattern/line metadata is kept
// for display; its owner list is extended with any owners unique to the
// secondary rule.
func unionRule(primary, secondary *codeowners.Rule) *codeowners.Rule {
	merged := *primary
	seen := make(map[string]bool, len(primary.Owners))
	owners := make([]codeowners.Owner, 0, len(primary.Owners)+len(secondary.Owners))
	for _, o := range primary.Owners {
		seen[o.String()] = true
		owners = append(owners, o)
	}
	for _, o := range secondary.Owners {
		if !seen[o.String()] {
			seen[o.String()] = true
			owners = append(owners, o)
		}
	}
	merged.Owners = owners
	return &merged
}

// whoseMine reports whether any of owners resolves to identity, and if so
// which owner matched.
//
// For a TeamOwner, Owner.Value is "org/team-slug" with no leading "@", and
// the second segment is the team's slug — not its display name. Comparing
// against team.Name ("Platform Eng" vs "platform-eng") would silently never
// match, so identity.TeamSlugs must be built from slugs, not names. GitHub
// treats logins, org names, and team slugs case-insensitively.
func whoseMine(owners []codeowners.Owner, identity Identity) (bool, string) {
	for _, o := range owners {
		switch o.Type {
		case codeowners.UsernameOwner:
			if identity.Login != "" && strings.EqualFold(o.Value, identity.Login) {
				return true, o.String()
			}
		case codeowners.TeamOwner:
			if identity.TeamSlugs[strings.ToLower(o.Value)] {
				return true, o.String()
			}
		case codeowners.EmailOwner:
			if identity.Emails[strings.ToLower(o.Value)] {
				return true, o.String()
			}
		}
	}
	return false, ""
}

// Group buckets file ownerships by owner, sorted mine-first, then by file
// count descending, then owner name; "(unowned)" always sorts last. A file
// with multiple owners appears in each of their groups.
func Group(files []FileOwnership) []OwnerGroup {
	byOwner := map[string]*OwnerGroup{}
	var order []string

	add := func(key string, mine bool, fo FileOwnership) {
		g, ok := byOwner[key]
		if !ok {
			g = &OwnerGroup{Owner: key}
			byOwner[key] = g
			order = append(order, key)
		}
		if mine {
			g.Mine = true
		}
		g.Files = append(g.Files, fo)
	}

	for _, fo := range files {
		if len(fo.Owners) == 0 {
			add("(unowned)", false, fo)
			continue
		}
		for _, o := range fo.Owners {
			add(o.String(), fo.Mine, fo)
		}
	}

	groups := make([]OwnerGroup, 0, len(order))
	for _, k := range order {
		groups = append(groups, *byOwner[k])
	}

	sort.SliceStable(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		if a.Owner == "(unowned)" || b.Owner == "(unowned)" {
			return b.Owner == "(unowned)" && a.Owner != "(unowned)"
		}
		if a.Mine != b.Mine {
			return a.Mine
		}
		if len(a.Files) != len(b.Files) {
			return len(a.Files) > len(b.Files)
		}
		return a.Owner < b.Owner
	})
	return groups
}

// Unowned returns the subset of files with no matching CODEOWNERS rule.
func Unowned(files []FileOwnership) []FileOwnership {
	var out []FileOwnership
	for _, fo := range files {
		if len(fo.Owners) == 0 {
			out = append(out, fo)
		}
	}
	return out
}
