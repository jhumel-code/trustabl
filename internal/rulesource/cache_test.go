package rulesource

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackDir_PerSHA(t *testing.T) {
	got := packDir("/cache", "abc123")
	want := filepath.Join("/cache", "abc123")
	if got != want {
		t.Errorf("packDir = %q, want %q", got, want)
	}
}

func TestPackExists(t *testing.T) {
	cache := t.TempDir()
	if packExists(cache, "abc123") {
		t.Error("packExists true for absent pack")
	}
	if err := os.MkdirAll(packDir(cache, "abc123"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !packExists(cache, "abc123") {
		t.Error("packExists false for present pack")
	}
}

func TestPruneCache_KeepsOnlyCurrent(t *testing.T) {
	cache := t.TempDir()
	for _, sha := range []string{"aaa", "bbb", "ccc"} {
		if err := os.MkdirAll(packDir(cache, sha), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A stale temp-clone dir (interrupted clone) should also be cleared.
	if err := os.MkdirAll(filepath.Join(cache, ".tmp-clone-123"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeCurrent(cache, "bbb"); err != nil {
		t.Fatal(err)
	}

	pruneCache(cache, "bbb")

	if !packExists(cache, "bbb") {
		t.Error("kept SHA bbb was removed")
	}
	if packExists(cache, "aaa") || packExists(cache, "ccc") {
		t.Error("stale pack dirs not pruned")
	}
	if _, err := os.Stat(filepath.Join(cache, ".tmp-clone-123")); !os.IsNotExist(err) {
		t.Error("stale temp-clone dir not pruned")
	}
	if sha, ok := readCurrent(cache); !ok || sha != "bbb" {
		t.Errorf("current pointer damaged: got (%q, %v), want (bbb, true)", sha, ok)
	}
}

func TestCurrentPointer_RoundTrip(t *testing.T) {
	cache := t.TempDir()
	if _, ok := readCurrent(cache); ok {
		t.Error("readCurrent ok=true on empty cache")
	}
	if err := writeCurrent(cache, "deadbeef"); err != nil {
		t.Fatalf("writeCurrent: %v", err)
	}
	sha, ok := readCurrent(cache)
	if !ok || sha != "deadbeef" {
		t.Errorf("readCurrent = (%q, %v), want (deadbeef, true)", sha, ok)
	}
}
