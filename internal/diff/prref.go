package diff

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
)

// PRRef is a parsed --pr value: a pull request number plus, when the value
// was a URL, the repository that PR lives in.
type PRRef struct {
	Number  int
	Repo    repository.Repository
	HasRepo bool
}

var prURLPathRE = regexp.MustCompile(`^/([^/]+)/([^/]+)/pull/(\d+)(?:/.*)?$`)

// ParsePRRef parses a --pr value: a bare number, "#number", or a pull
// request URL. A URL also yields the repository it lives in.
func ParsePRRef(s string) (PRRef, error) {
	if n, err := strconv.Atoi(strings.TrimPrefix(s, "#")); err == nil {
		return PRRef{Number: n}, nil
	}

	urlStr := s
	if !strings.Contains(urlStr, "://") && strings.Contains(urlStr, "/pull/") {
		urlStr = "https://" + urlStr
	}

	u, err := url.Parse(urlStr)
	if err == nil && u.Host != "" {
		if m := prURLPathRE.FindStringSubmatch(u.Path); m != nil {
			number, numErr := strconv.Atoi(m[3])
			if numErr == nil {
				host := normalizeHost(u.Host)
				if host == "" {
					return PRRef{}, fmt.Errorf("invalid --pr value %q: expected a pull request number or URL", s)
				}
				return PRRef{
					Number: number,
					Repo: repository.Repository{
						Host:  host,
						Owner: m[1],
						Name:  m[2],
					},
					HasRepo: true,
				}, nil
			}
		}
	}

	return PRRef{}, fmt.Errorf("invalid --pr value %q: expected a pull request number or URL", s)
}

func normalizeHost(h string) string {
	return strings.TrimPrefix(strings.ToLower(h), "www.")
}
