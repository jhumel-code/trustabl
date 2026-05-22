package rulesource

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// newFixtureRepo creates a non-bare git repo at dir with one commit holding
// the given files, and returns the commit SHA. Used as a local "remote".
func newFixtureRepo(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, err := wt.Add(name); err != nil {
			t.Fatalf("add %s: %v", name, err)
		}
	}
	h, err := wt.Commit("fixture", &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	return h.String()
}

func TestResolveRef_DefaultHEAD(t *testing.T) {
	dir := t.TempDir()
	remote := filepath.Join(dir, "remote")
	want := newFixtureRepo(t, remote, map[string]string{"manifest.yaml": "schema_version: 1\n"})
	sha, _, err := resolveRef(remote, "")
	if err != nil {
		t.Fatalf("resolveRef: %v", err)
	}
	if sha != want {
		t.Errorf("sha = %q, want %q", sha, want)
	}
}

func TestCloneInto_CopiesContent(t *testing.T) {
	dir := t.TempDir()
	remote := filepath.Join(dir, "remote")
	newFixtureRepo(t, remote, map[string]string{
		"manifest.yaml":     "schema_version: 1\n",
		"claude_sdk/a.yaml": "policy: {}\n",
	})
	dest := filepath.Join(dir, "clone")
	_, name, err := resolveRef(remote, "")
	if err != nil {
		t.Fatalf("resolveRef: %v", err)
	}
	if err := cloneInto(remote, name, dest); err != nil {
		t.Fatalf("cloneInto: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "manifest.yaml")); err != nil {
		t.Errorf("manifest.yaml not cloned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "claude_sdk", "a.yaml")); err != nil {
		t.Errorf("claude_sdk/a.yaml not cloned: %v", err)
	}
}

func TestResolveRef_NetworkError(t *testing.T) {
	if _, _, err := resolveRef(filepath.Join(t.TempDir(), "does-not-exist"), ""); err == nil {
		t.Fatal("expected error for nonexistent remote, got nil")
	}
}
