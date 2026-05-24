package ingestion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectSDKDeps_GoogleADK(t *testing.T) {
	dir := t.TempDir()
	pyproject := `[project]
dependencies = ["google-adk>=0.1.0"]
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := detectSDKDeps(dir)
	var found bool
	for _, d := range deps {
		if d.Name == "google-adk" && d.Source == "pyproject.toml" {
			found = true
		}
	}
	if !found {
		t.Errorf("google-adk not in detected deps: %+v", deps)
	}
}
