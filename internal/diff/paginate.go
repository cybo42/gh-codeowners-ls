package diff

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

var linkHeaderRE = regexp.MustCompile(`<([^>]+)>;\s*rel="([^"]+)"`)

// Paginate GETs path, decodes each page as a JSON array of T, and follows the
// Link header's rel="next" URL until the API stops returning one.
func Paginate[T any](client *api.RESTClient, path string) ([]T, error) {
	var all []T
	for path != "" {
		resp, err := client.Request(http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		var page []T
		decodeErr := json.NewDecoder(resp.Body).Decode(&page)
		next := nextPageURL(resp.Header.Get("Link"))
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode %s: %w", path, decodeErr)
		}

		all = append(all, page...)
		path = next
	}
	return all, nil
}

func nextPageURL(header string) string {
	if header == "" {
		return ""
	}
	for part := range strings.SplitSeq(header, ",") {
		m := linkHeaderRE.FindStringSubmatch(strings.TrimSpace(part))
		if len(m) == 3 && m[2] == "next" {
			return m[1]
		}
	}
	return ""
}
