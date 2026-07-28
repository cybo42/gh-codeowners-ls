package diff

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(body, linkHeader string) *http.Response {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	if linkHeader != "" {
		h.Set("Link", linkHeader)
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     h,
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
