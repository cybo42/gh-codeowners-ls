package output

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/cybo42/gh-codeowners-ls/internal/owners"
)

type jsonOwner struct {
	Value string `json:"value"`
	Type  string `json:"type"`
	Name  string `json:"name"`
}

type jsonRule struct {
	Pattern    string `json:"pattern"`
	LineNumber int    `json:"lineNumber"`
}

type jsonFile struct {
	Path         string      `json:"path"`
	PreviousPath string      `json:"previousPath,omitempty"`
	Status       string      `json:"status"`
	Additions    int         `json:"additions"`
	Deletions    int         `json:"deletions"`
	Owners       []jsonOwner `json:"owners"`
	Rule         *jsonRule   `json:"rule,omitempty"`
	Mine         bool        `json:"mine"`
	MineVia      string      `json:"mineVia,omitempty"`
}

type jsonOwnerGroup struct {
	Owner string   `json:"owner"`
	Mine  bool     `json:"mine"`
	Files []string `json:"files"`
}

type jsonIdentity struct {
	Login      string   `json:"login"`
	Teams      []string `json:"teams"`
	TeamsKnown bool     `json:"teamsKnown"`
	Warning    string   `json:"warning,omitempty"`
}

type jsonRepo struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

type jsonSource struct {
	Kind      string `json:"kind"`
	PR        int    `json:"pr,omitempty"`
	Base      string `json:"base"`
	Head      string `json:"head"`
	Label     string `json:"label"`
	Truncated bool   `json:"truncated"`
}

type jsonReport struct {
	Repo           jsonRepo                 `json:"repo"`
	Source         jsonSource               `json:"source"`
	Identity       jsonIdentity             `json:"identity"`
	CodeownersPath string                   `json:"codeownersPath,omitempty"`
	Files          []jsonFile               `json:"files"`
	ByOwner        []jsonOwnerGroup         `json:"byOwner"`
	Unowned        []string                 `json:"unowned"`
	Validation     []owners.ValidationError `json:"validation,omitempty"`
}

// JSON writes report as the --json machine-readable schema.
func JSON(w io.Writer, report owners.Report) error {
	out := jsonReport{
		Repo: jsonRepo{
			Host:  report.Repo.Host,
			Owner: report.Repo.Owner,
			Name:  report.Repo.Name,
		},
		Source: jsonSource{
			Kind:      string(report.Source.Kind),
			PR:        report.Source.PR,
			Base:      report.Source.Base,
			Head:      report.Source.Head,
			Label:     report.Source.Label,
			Truncated: report.Source.Truncated,
		},
		Identity: jsonIdentity{
			Login:      report.Identity.Login,
			Teams:      sortedKeys(report.Identity.TeamSlugs),
			TeamsKnown: report.Identity.TeamsKnown,
			Warning:    report.Identity.Warning,
		},
		CodeownersPath: report.CodeownersPath,
		Validation:     report.Validation,
		Files:          make([]jsonFile, 0, len(report.Files)),
		ByOwner:        make([]jsonOwnerGroup, 0, len(report.ByOwner)),
		Unowned:        make([]string, 0, len(report.Unowned)),
	}

	for _, fo := range report.Files {
		out.Files = append(out.Files, toJSONFile(fo))
	}
	for _, g := range report.ByOwner {
		paths := make([]string, len(g.Files))
		for i, f := range g.Files {
			paths[i] = f.File.Path
		}
		out.ByOwner = append(out.ByOwner, jsonOwnerGroup{Owner: g.Owner, Mine: g.Mine, Files: paths})
	}
	for _, fo := range report.Unowned {
		out.Unowned = append(out.Unowned, fo.File.Path)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func toJSONFile(fo owners.FileOwnership) jsonFile {
	jf := jsonFile{
		Path:         fo.File.Path,
		PreviousPath: fo.File.PreviousPath,
		Status:       string(fo.File.Status),
		Additions:    fo.File.Additions,
		Deletions:    fo.File.Deletions,
		Mine:         fo.Mine,
		MineVia:      fo.MineVia,
		Owners:       make([]jsonOwner, 0, len(fo.Owners)),
	}
	for _, o := range fo.Owners {
		jf.Owners = append(jf.Owners, jsonOwner{Value: o.Value, Type: o.Type, Name: o.String()})
	}
	if fo.Rule != nil {
		jf.Rule = &jsonRule{Pattern: fo.Rule.RawPattern(), LineNumber: fo.Rule.LineNumber}
	}
	return jf
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
