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
