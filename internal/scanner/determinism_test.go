package scanner_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/trustabl/trustabl/internal/scanner"
)

// TestScanDeterministic asserts that two runs over the same fixture produce
// byte-identical artifacts and the same ScanID. Guards the contract
// documented in ARCHITECTURE.md §7.
func TestScanDeterministic(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	fixture := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "deterministic-fixture")

	r1, err := scanner.Run(scanner.Config{Target: fixture})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	r2, err := scanner.Run(scanner.Config{Target: fixture})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	if r1.ScanID != r2.ScanID {
		t.Errorf("ScanID drifted: %q vs %q", r1.ScanID, r2.ScanID)
	}
	if len(r1.GeneratedArtifacts) != len(r2.GeneratedArtifacts) {
		t.Fatalf("artifact count differs: %d vs %d", len(r1.GeneratedArtifacts), len(r2.GeneratedArtifacts))
	}
	for i, a1 := range r1.GeneratedArtifacts {
		a2 := r2.GeneratedArtifacts[i]
		if a1.RelativePath != a2.RelativePath {
			t.Errorf("artifact %d path differs: %q vs %q", i, a1.RelativePath, a2.RelativePath)
			continue
		}
		if a1.Contents != a2.Contents {
			t.Errorf("artifact %q content not byte-equal across runs (len %d vs %d)",
				a1.RelativePath, len(a1.Contents), len(a2.Contents))
		}
	}
}
