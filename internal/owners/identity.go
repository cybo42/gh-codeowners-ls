package owners

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"

	"github.com/cybo42/gh-codeowners-ls/internal/diff"
)

type userResponse struct {
	Login string `json:"login"`
}

type teamResponse struct {
	Slug         string `json:"slug"`
	Organization struct {
		Login string `json:"login"`
	} `json:"organization"`
}

type emailResponse struct {
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
}

// ResolveIdentity determines who the authenticated gh user is: their login,
// GitHub team memberships scoped to repo's owner org (child-team membership
// is already resolved server-side by /user/teams, so there's no need to walk
// parent teams), and verified emails. Missing read:org or user:email scope
// degrades gracefully — those signals are just left unknown — rather than
// failing the whole command.
func ResolveIdentity(client *api.RESTClient, repo repository.Repository) (Identity, error) {
	identity := Identity{
		TeamSlugs: map[string]bool{},
		Emails:    map[string]bool{},
	}

	var user userResponse
	if err := client.Get("user", &user); err != nil {
		return identity, fmt.Errorf("resolve authenticated user: %w", err)
	}
	identity.Login = user.Login

	teams, err := diff.Paginate[teamResponse](client, "user/teams?per_page=100")
	if err != nil {
		identity.Warning = scopeWarning(err, "read:org", "team memberships")
	} else {
		identity.TeamsKnown = true
		for _, t := range teams {
			if strings.EqualFold(t.Organization.Login, repo.Owner) {
				identity.TeamSlugs[strings.ToLower(t.Organization.Login+"/"+t.Slug)] = true
			}
		}
	}

	emails, err := diff.Paginate[emailResponse](client, "user/emails?per_page=100")
	if err != nil {
		if identity.Warning == "" {
			identity.Warning = scopeWarning(err, "user:email", "verified emails")
		}
	} else {
		for _, e := range emails {
			if e.Verified {
				identity.Emails[strings.ToLower(e.Email)] = true
			}
		}
	}

	return identity, nil
}

func scopeWarning(err error, scope, what string) string {
	if httpErr, ok := err.(*api.HTTPError); ok && (httpErr.StatusCode == http.StatusForbidden || httpErr.StatusCode == http.StatusNotFound) {
		return fmt.Sprintf("could not resolve %s (missing %q scope) — run `gh auth refresh -s %s`", what, scope, scope)
	}
	return fmt.Sprintf("could not resolve %s: %v", what, err)
}
