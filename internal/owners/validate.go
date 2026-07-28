package owners

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

type validateResponse struct {
	Errors []ValidationError `json:"errors"`
}

// Validate fetches CODEOWNERS syntax/permission errors GitHub itself would
// report for ref. A 404 means there's no CODEOWNERS file on that ref, which
// is reported as no errors rather than a failure.
func Validate(client *api.RESTClient, repo repository.Repository, ref string) ([]ValidationError, error) {
	path := fmt.Sprintf("repos/%s/%s/codeowners/errors?ref=%s", repo.Owner, repo.Name, url.QueryEscape(ref))
	var resp validateResponse
	if err := client.Get(path, &resp); err != nil {
		if httpErr, ok := err.(*api.HTTPError); ok && httpErr.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("validate CODEOWNERS: %w", err)
	}
	return resp.Errors, nil
}
