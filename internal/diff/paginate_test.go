package diff

import (
	"net/http"
	"testing"
)

func TestPaginate_FollowsLinkHeader(t *testing.T) {
	type item struct {
		Name string `json:"name"`
	}

	calls := 0
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		switch calls {
		case 1:
			if got := req.URL.Path; got != "/repos/acme/widgets/pulls/1/files" {
				t.Fatalf("unexpected path on first call: %s", got)
			}
			return jsonResponse(`[{"name":"a"},{"name":"b"}]`,
				`<https://api.github.com/repos/acme/widgets/pulls/1/files?page=2>; rel="next", <https://api.github.com/repos/acme/widgets/pulls/1/files?page=2>; rel="last"`), nil
		case 2:
			if got := req.URL.RawQuery; got != "page=2" {
				t.Fatalf("unexpected query on second call: %s", got)
			}
			return jsonResponse(`[{"name":"c"}]`, ""), nil
		default:
			t.Fatalf("unexpected call #%d", calls)
			return nil, nil
		}
	})

	client := newTestClient(t, rt)
	items, err := Paginate[item](client, "repos/acme/widgets/pulls/1/files")
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 requests, got %d", calls)
	}

	got := []string{items[0].Name, items[1].Name, items[2].Name}
	want := []string{"a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("items = %v, want %v", got, want)
		}
	}
}

func TestPaginate_NoLinkHeaderStopsAfterOnePage(t *testing.T) {
	type item struct {
		Name string `json:"name"`
	}

	calls := 0
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(`[{"name":"solo"}]`, ""), nil
	})

	client := newTestClient(t, rt)
	items, err := Paginate[item](client, "repos/acme/widgets/pulls/1/files")
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 request, got %d", calls)
	}
	if len(items) != 1 || items[0].Name != "solo" {
		t.Fatalf("items = %v", items)
	}
}
