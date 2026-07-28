package diff

import (
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

// maxPRFiles is GitHub's ceiling on how many files a PR's files API will
// return, even across pagination.
const maxPRFiles = 3000

type prFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
}

type prMetadata struct {
	Title string `json:"title"`
	Base  struct {
		Ref string `json:"ref"`
	} `json:"base"`
	Head struct {
		Ref string `json:"ref"`
	} `json:"head"`
}

// FilesForPR fetches a pull request's changed-file list via the REST API,
// following pagination until exhausted or the API's file-count ceiling is
// hit.
func FilesForPR(client *api.RESTClient, repo repository.Repository, number int) (Source, error) {
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/files?per_page=100", repo.Owner, repo.Name, number)
	raw, err := Paginate[prFile](client, path)
	if err != nil {
		return Source{}, fmt.Errorf("fetch files for PR #%d: %w", number, err)
	}

	files := make([]ChangedFile, 0, len(raw))
	for _, f := range raw {
		files = append(files, ChangedFile{
			Path:         f.Filename,
			PreviousPath: f.PreviousFilename,
			Status:       prStatus(f.Status),
			Additions:    f.Additions,
			Deletions:    f.Deletions,
		})
	}

	meta, err := fetchPRMetadata(client, repo, number)
	if err != nil {
		return Source{}, err
	}

	return Source{
		Kind:      SourcePR,
		PR:        number,
		Base:      meta.Base.Ref,
		Head:      meta.Head.Ref,
		Label:     fmt.Sprintf("PR #%d %q", number, meta.Title),
		Files:     files,
		Truncated: len(files) >= maxPRFiles,
	}, nil
}

func fetchPRMetadata(client *api.RESTClient, repo repository.Repository, number int) (prMetadata, error) {
	var meta prMetadata
	path := fmt.Sprintf("repos/%s/%s/pulls/%d", repo.Owner, repo.Name, number)
	if err := client.Get(path, &meta); err != nil {
		return prMetadata{}, fmt.Errorf("fetch PR #%d metadata: %w", number, err)
	}
	return meta, nil
}

func prStatus(s string) Status {
	switch s {
	case "added":
		return StatusAdded
	case "removed":
		return StatusRemoved
	case "renamed":
		return StatusRenamed
	case "copied":
		return StatusCopied
	default:
		return StatusModified
	}
}
