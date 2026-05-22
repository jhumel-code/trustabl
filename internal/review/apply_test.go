package review_test

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/trustabl/trustabl/internal/models"
	"github.com/trustabl/trustabl/internal/review"
)

func sampleArtifacts() []models.GeneratedArtifact {
	return []models.GeneratedArtifact{
		{RelativePath: "hooks/pretooluse_validate.py", Contents: "# validate\n"},
		{RelativePath: "openshell/policy.yaml", Contents: "apiVersion: x\n"},
	}
}

func TestApplyArtifactsWritesFiles(t *testing.T) {
	root := t.TempDir()
	if err := review.ApplyArtifacts(root, sampleArtifacts(), false); err != nil {
		t.Fatalf("ApplyArtifacts: %v", err)
	}
	for _, a := range sampleArtifacts() {
		got, err := os.ReadFile(filepath.Join(root, a.RelativePath))
		if err != nil {
			t.Fatalf("reading %s: %v", a.RelativePath, err)
		}
		if string(got) != a.Contents {
			t.Fatalf("%s contents = %q, want %q", a.RelativePath, got, a.Contents)
		}
	}
}

func TestApplyArtifactsRefusesOverwrite(t *testing.T) {
	root := t.TempDir()
	// First write succeeds.
	if err := review.ApplyArtifacts(root, sampleArtifacts(), false); err != nil {
		t.Fatalf("first ApplyArtifacts: %v", err)
	}
	// Second write without overwrite must refuse.
	err := review.ApplyArtifacts(root, sampleArtifacts(), false)
	if err == nil {
		t.Fatal("ApplyArtifacts: expected refuse-to-overwrite error, got nil")
	}
	// And it must NOT have clobbered the existing file.
	existing := filepath.Join(root, "hooks/pretooluse_validate.py")
	if err := os.WriteFile(existing, []byte("MODIFIED"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := review.ApplyArtifacts(root, sampleArtifacts(), false); err == nil {
		t.Fatal("ApplyArtifacts: expected refusal on pre-existing file")
	}
	got, _ := os.ReadFile(existing)
	if string(got) != "MODIFIED" {
		t.Fatalf("refused apply still modified file: got %q", got)
	}
}

func TestApplyArtifactsOverwriteAllowed(t *testing.T) {
	root := t.TempDir()
	if err := review.ApplyArtifacts(root, sampleArtifacts(), false); err != nil {
		t.Fatalf("first ApplyArtifacts: %v", err)
	}
	changed := []models.GeneratedArtifact{
		{RelativePath: "hooks/pretooluse_validate.py", Contents: "# changed\n"},
	}
	if err := review.ApplyArtifacts(root, changed, true); err != nil {
		t.Fatalf("ApplyArtifacts overwrite: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "hooks/pretooluse_validate.py"))
	if string(got) != "# changed\n" {
		t.Fatalf("overwrite did not take: got %q", got)
	}
}

func TestExportZIP(t *testing.T) {
	out := filepath.Join(t.TempDir(), "bundle.zip")
	arts := sampleArtifacts()
	if err := review.ExportZIP(out, arts); err != nil {
		t.Fatalf("ExportZIP: %v", err)
	}

	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatalf("opening zip: %v", err)
	}
	defer zr.Close()

	found := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		found[f.Name] = string(b)
	}
	if len(found) != len(arts) {
		t.Fatalf("zip has %d entries, want %d", len(found), len(arts))
	}
	for _, a := range arts {
		if found[a.RelativePath] != a.Contents {
			t.Fatalf("zip entry %s = %q, want %q", a.RelativePath, found[a.RelativePath], a.Contents)
		}
	}
}

func TestExportZIPEmptyErrors(t *testing.T) {
	out := filepath.Join(t.TempDir(), "empty.zip")
	if err := review.ExportZIP(out, nil); err == nil {
		t.Fatal("ExportZIP(nil): expected error, got nil")
	}
}
