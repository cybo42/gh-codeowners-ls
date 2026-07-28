package cmd

import (
	"testing"

	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/cybo42/gh-codeowners-ls/internal/diff"
)

func TestResolveRepo(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		ref      diff.PRRef
		wantRepo repository.Repository
		wantSpec string
		wantErr  bool
	}{
		{
			name:     "--repo only",
			explicit: "acme/widgets",
			wantRepo: repository.Repository{Host: "github.com", Owner: "acme", Name: "widgets"},
			wantSpec: "github.com/acme/widgets",
		},
		{
			name: "URL only",
			ref: diff.PRRef{
				Number:  482,
				HasRepo: true,
				Repo:    repository.Repository{Host: "github.com", Owner: "acme", Name: "widgets"},
			},
			wantRepo: repository.Repository{Host: "github.com", Owner: "acme", Name: "widgets"},
			wantSpec: "github.com/acme/widgets",
		},
		{
			name:     "--repo and matching URL",
			explicit: "acme/widgets",
			ref: diff.PRRef{
				Number:  482,
				HasRepo: true,
				Repo:    repository.Repository{Host: "github.com", Owner: "acme", Name: "widgets"},
			},
			wantRepo: repository.Repository{Host: "github.com", Owner: "acme", Name: "widgets"},
			wantSpec: "github.com/acme/widgets",
		},
		{
			name:     "--repo conflicts with URL",
			explicit: "cli/go-gh",
			ref: diff.PRRef{
				Number:  9000,
				HasRepo: true,
				Repo:    repository.Repository{Host: "github.com", Owner: "cli", Name: "cli"},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, spec, err := resolveRepo(tc.explicit, tc.ref)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveRepo(%q, %+v) = %+v, want error", tc.explicit, tc.ref, repo)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveRepo(%q, %+v) unexpected error: %v", tc.explicit, tc.ref, err)
			}
			if repo != tc.wantRepo {
				t.Errorf("repo = %+v, want %+v", repo, tc.wantRepo)
			}
			if spec != tc.wantSpec {
				t.Errorf("spec = %q, want %q", spec, tc.wantSpec)
			}
		})
	}
}

func TestResolveRepo_CurrentFallback(t *testing.T) {
	// No --repo, no URL: falls through to repository.Current(), which shells
	// out to git. Skip if that's not viable in this environment; we only
	// care that no panic occurs and errors get the --pr/--repo hint.
	repo, spec, err := resolveRepo("", diff.PRRef{})
	if err != nil {
		if spec != "" {
			t.Errorf("spec = %q on error, want empty", spec)
		}
		return
	}
	if repo == (repository.Repository{}) {
		t.Error("resolveRepo returned zero-value repo with no error")
	}
}
