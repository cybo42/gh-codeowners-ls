package diff

import (
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/repository"
)

func TestFilesForPR(t *testing.T) {
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/files"):
			return jsonResponse(`[
				{"filename":"a.go","status":"modified","additions":1,"deletions":2},
				{"filename":"new.go","previous_filename":"old.go","status":"renamed"}
			]`, ""), nil
		case strings.HasSuffix(req.URL.Path, "/pulls/42"):
			return jsonResponse(`{"title":"Add billing","base":{"ref":"main"},"head":{"ref":"feature"}}`, ""), nil
		default:
			t.Fatalf("unexpected request to %s", req.URL.String())
			return nil, nil
		}
	})

	client := newTestClient(t, rt)
	repo := repository.Repository{Owner: "acme", Name: "widgets"}

	src, err := FilesForPR(client, repo, 42)
	if err != nil {
		t.Fatalf("FilesForPR: %v", err)
	}

	if src.Label != `PR #42 "Add billing"` {
		t.Errorf("Label = %q", src.Label)
	}
	if src.Base != "main" || src.Head != "feature" {
		t.Errorf("Base/Head = %s/%s, want main/feature", src.Base, src.Head)
	}
	if len(src.Files) != 2 {
		t.Fatalf("Files = %d, want 2", len(src.Files))
	}
	if src.Files[0].Status != StatusModified || src.Files[0].Additions != 1 || src.Files[0].Deletions != 2 {
		t.Errorf("Files[0] = %+v", src.Files[0])
	}
	if src.Files[1].Status != StatusRenamed || src.Files[1].PreviousPath != "old.go" || src.Files[1].Path != "new.go" {
		t.Errorf("Files[1] rename not parsed: %+v", src.Files[1])
	}
	if src.Truncated {
		t.Errorf("Truncated = true, want false for 2 files")
	}
}
