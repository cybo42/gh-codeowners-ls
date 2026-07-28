package owners

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := f(req)
	if resp != nil {
		resp.Request = req // HandleHTTPError dereferences this for non-2xx responses
	}
	return resp, err
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func newTestClient(t *testing.T, rt http.RoundTripper) *api.RESTClient {
	t.Helper()
	client, err := api.NewRESTClient(api.ClientOptions{
		Host:      "github.com",
		AuthToken: "test-token",
		Transport: rt,
	})
	if err != nil {
		t.Fatalf("NewRESTClient: %v", err)
	}
	return client
}

func TestResolveIdentity_HappyPath(t *testing.T) {
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/user"):
			return jsonResponse(200, `{"login":"alice"}`), nil
		case strings.Contains(req.URL.Path, "/user/teams"):
			return jsonResponse(200, `[
				{"slug":"platform-eng","name":"Platform Eng","organization":{"login":"acme"}},
				{"slug":"other-team","name":"Other Team","organization":{"login":"other-org"}}
			]`), nil
		case strings.Contains(req.URL.Path, "/user/emails"):
			return jsonResponse(200, `[
				{"email":"alice@acme.com","verified":true},
				{"email":"unverified@acme.com","verified":false}
			]`), nil
		default:
			t.Fatalf("unexpected request to %s", req.URL.String())
			return nil, nil
		}
	})

	client := newTestClient(t, rt)
	repo := repository.Repository{Owner: "acme", Name: "widgets"}

	identity, err := ResolveIdentity(client, repo)
	if err != nil {
		t.Fatalf("ResolveIdentity: %v", err)
	}

	if identity.Login != "alice" {
		t.Errorf("Login = %q, want alice", identity.Login)
	}
	if !identity.TeamsKnown {
		t.Errorf("TeamsKnown = false, want true")
	}
	if identity.Warning != "" {
		t.Errorf("Warning = %q, want empty", identity.Warning)
	}

	// The matching trap: identity.TeamSlugs must be built from the team's
	// slug ("platform-eng"), not its display name ("Platform Eng"), and
	// must be scoped to the repo's owner org ("acme"), excluding "other-org".
	if !identity.TeamSlugs["acme/platform-eng"] {
		t.Errorf("TeamSlugs = %v, want acme/platform-eng present (built from slug)", identity.TeamSlugs)
	}
	if identity.TeamSlugs["acme/platform eng"] || identity.TeamSlugs["acme/Platform Eng"] {
		t.Errorf("TeamSlugs = %v, must not be built from display name", identity.TeamSlugs)
	}
	if identity.TeamSlugs["other-org/other-team"] {
		t.Errorf("TeamSlugs = %v, want other-org team excluded", identity.TeamSlugs)
	}

	if !identity.Emails["alice@acme.com"] {
		t.Errorf("Emails = %v, want alice@acme.com present", identity.Emails)
	}
	if identity.Emails["unverified@acme.com"] {
		t.Errorf("Emails = %v, want unverified email excluded", identity.Emails)
	}
}

func TestResolveIdentity_DegradesOnMissingScope(t *testing.T) {
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/user"):
			return jsonResponse(200, `{"login":"alice"}`), nil
		case strings.Contains(req.URL.Path, "/user/teams"):
			return jsonResponse(403, `{"message":"Resource not accessible by integration"}`), nil
		case strings.Contains(req.URL.Path, "/user/emails"):
			return jsonResponse(200, `[]`), nil
		default:
			t.Fatalf("unexpected request to %s", req.URL.String())
			return nil, nil
		}
	})

	client := newTestClient(t, rt)
	repo := repository.Repository{Owner: "acme", Name: "widgets"}

	identity, err := ResolveIdentity(client, repo)
	if err != nil {
		t.Fatalf("ResolveIdentity returned an error instead of degrading: %v", err)
	}

	if identity.Login != "alice" {
		t.Errorf("Login = %q, want alice (login-only matching should still work)", identity.Login)
	}
	if identity.TeamsKnown {
		t.Errorf("TeamsKnown = true, want false after a 403 on /user/teams")
	}
	if identity.Warning == "" {
		t.Errorf("Warning = %q, want a non-empty warning naming the fix", identity.Warning)
	}
	if !strings.Contains(identity.Warning, "read:org") {
		t.Errorf("Warning = %q, want it to mention the read:org scope", identity.Warning)
	}
}
