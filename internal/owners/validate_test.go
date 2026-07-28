package owners

import (
	"net/http"
	"testing"

	"github.com/cli/go-gh/v2/pkg/repository"
)

func TestValidate_ReturnsErrors(t *testing.T) {
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(200, `{"errors":[{"line":2,"column":14,"kind":"Unknown owner","message":"Unknown owner on line 2","path":".github/CODEOWNERS","source":"...","suggestion":"make sure the owner exists"}]}`), nil
	})

	client := newTestClient(t, rt)
	repo := repository.Repository{Owner: "acme", Name: "widgets"}

	errs, err := Validate(client, repo, "main")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(errs) != 1 {
		t.Fatalf("Validate() = %d errors, want 1", len(errs))
	}
	if errs[0].Line != 2 || errs[0].Kind != "Unknown owner" {
		t.Errorf("Validate()[0] = %+v", errs[0])
	}
}

func TestValidate_404MeansNoErrors(t *testing.T) {
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(404, `{"message":"Not Found"}`), nil
	})

	client := newTestClient(t, rt)
	repo := repository.Repository{Owner: "acme", Name: "widgets"}

	errs, err := Validate(client, repo, "main")
	if err != nil {
		t.Fatalf("Validate: unexpected error for a 404: %v", err)
	}
	if errs != nil {
		t.Errorf("Validate() = %v, want nil for a 404 (no CODEOWNERS on that ref)", errs)
	}
}
