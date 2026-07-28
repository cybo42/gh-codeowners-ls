package diff

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func mustRunGit(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out.String())
	}
	return strings.TrimSpace(out.String())
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLocalFiles_RealGitRepo builds a throwaway repo with real git
// init/commits and asserts on the parsed ChangedFiles, including a rename
// and a path containing a space (the reason `-z` is required).
func TestLocalFiles_RealGitRepo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	mustRunGit(t, "init", "-q", "-b", "main")

	writeFile(t, dir, "keep.txt", "unchanged\n")
	writeFile(t, dir, "old-name.txt", strings.Repeat("rename me please\n", 5))
	mustRunGit(t, "add", "-A")
	mustRunGit(t, "commit", "-q", "-m", "base")
	base := mustRunGit(t, "rev-parse", "HEAD")

	writeFile(t, dir, "keep.txt", "unchanged\nplus one more line\n")
	writeFile(t, dir, "added.txt", "new file\n")
	writeFile(t, dir, "with space.txt", "hello\n")
	if err := os.Rename(filepath.Join(dir, "old-name.txt"), filepath.Join(dir, "new-name.txt")); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, "add", "-A")
	mustRunGit(t, "commit", "-q", "-m", "changes")

	files, err := LocalFiles(LocalDiffOptions{Base: base, Head: "HEAD"})
	if err != nil {
		t.Fatalf("LocalFiles: %v", err)
	}

	byPath := map[string]ChangedFile{}
	for _, f := range files {
		byPath[f.Path] = f
	}

	if f, ok := byPath["keep.txt"]; !ok || f.Status != StatusModified {
		t.Errorf("keep.txt = %+v, ok=%v, want Modified", f, ok)
	}
	if f, ok := byPath["added.txt"]; !ok || f.Status != StatusAdded {
		t.Errorf("added.txt = %+v, ok=%v, want Added", f, ok)
	}
	if f, ok := byPath["with space.txt"]; !ok || f.Status != StatusAdded {
		t.Errorf("path with space not parsed correctly: %+v, ok=%v", f, ok)
	}
	if f, ok := byPath["new-name.txt"]; !ok || f.Status != StatusRenamed || f.PreviousPath != "old-name.txt" {
		t.Errorf("rename not detected: %+v, ok=%v", f, ok)
	}
}

func TestMergeBaseAndRepoRoot(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	mustRunGit(t, "init", "-q", "-b", "main")
	writeFile(t, dir, "f.txt", "1\n")
	mustRunGit(t, "add", "-A")
	mustRunGit(t, "commit", "-q", "-m", "c1")
	base := mustRunGit(t, "rev-parse", "HEAD")

	mustRunGit(t, "checkout", "-q", "-b", "feature")
	writeFile(t, dir, "f.txt", "2\n")
	mustRunGit(t, "add", "-A")
	mustRunGit(t, "commit", "-q", "-m", "c2")

	mb, err := MergeBase("main", "feature")
	if err != nil {
		t.Fatalf("MergeBase: %v", err)
	}
	if mb != base {
		t.Errorf("MergeBase = %s, want %s", mb, base)
	}

	root, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if root != resolvedDir {
		t.Errorf("RepoRoot = %s, want %s", root, resolvedDir)
	}
}
