package owners

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/hmarr/codeowners"

	"github.com/cybo42/gh-codeowners-ls/internal/diff"
)

// standardLocations mirrors the locations GitHub itself looks for CODEOWNERS.
var standardLocations = []string{"CODEOWNERS", ".github/CODEOWNERS", ".gitlab/CODEOWNERS", "docs/CODEOWNERS"}

// LoadOptions configures where to load a CODEOWNERS ruleset from.
type LoadOptions struct {
	ExplicitPath string
	Client       *api.RESTClient
	Repo         repository.Repository
	// BaseRef, when set (PR sources), fetches CODEOWNERS from that ref via
	// the API — that's the file GitHub itself evaluates for the PR — falling
	// back to the working tree with a stderr note on failure.
	BaseRef string
}

// LoadRuleset loads a CODEOWNERS ruleset and returns it alongside a display
// label for where it came from. An empty ruleset with no error means no
// CODEOWNERS file exists anywhere — every file will resolve as unowned.
func LoadRuleset(opts LoadOptions) (codeowners.Ruleset, string, error) {
	if opts.ExplicitPath != "" {
		rs, err := codeowners.LoadFile(opts.ExplicitPath)
		if err != nil {
			return nil, "", fmt.Errorf("load CODEOWNERS from %s: %w", opts.ExplicitPath, err)
		}
		return rs, opts.ExplicitPath, nil
	}

	if opts.BaseRef != "" && opts.Client != nil {
		rs, path, err := loadFromRef(opts.Client, opts.Repo, opts.BaseRef)
		if err == nil {
			return rs, path, nil
		}
		fmt.Fprintf(os.Stderr, "codeowners-ls: could not read CODEOWNERS from base ref %s (%v); falling back to working tree\n", opts.BaseRef, err)
	}

	return loadFromWorkingTree()
}

func loadFromWorkingTree() (codeowners.Ruleset, string, error) {
	root, _ := diff.RepoRoot() // ignore error: fall back to searching relative to cwd

	path := findStandardLocation(root)
	if path == "" {
		return codeowners.Ruleset{}, "", nil
	}

	rs, err := codeowners.LoadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("load CODEOWNERS from %s: %w", path, err)
	}

	label := path
	if root != "" {
		if rel, err := filepath.Rel(root, path); err == nil {
			label = rel
		}
	}
	return rs, label, nil
}

func findStandardLocation(repoRoot string) string {
	for _, p := range standardLocations {
		full := filepath.Join(repoRoot, p)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			return full
		}
	}
	return ""
}

func loadFromRef(client *api.RESTClient, repo repository.Repository, ref string) (codeowners.Ruleset, string, error) {
	for _, path := range standardLocations {
		body, err := fetchContents(client, repo, path, ref)
		if err != nil {
			continue
		}
		rs, err := codeowners.ParseFile(bytes.NewReader(body))
		if err != nil {
			return nil, "", fmt.Errorf("parse CODEOWNERS at %s@%s: %w", path, ref, err)
		}
		return rs, path, nil
	}
	return nil, "", fmt.Errorf("no CODEOWNERS found at any standard location on ref %s", ref)
}

type contentsResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func fetchContents(client *api.RESTClient, repo repository.Repository, path, ref string) ([]byte, error) {
	apiPath := fmt.Sprintf("repos/%s/%s/contents/%s?ref=%s", repo.Owner, repo.Name, path, url.QueryEscape(ref))
	var resp contentsResponse
	if err := client.Get(apiPath, &resp); err != nil {
		return nil, err
	}
	if resp.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding %q for %s", resp.Encoding, path)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(resp.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("decode contents of %s: %w", path, err)
	}
	return decoded, nil
}
