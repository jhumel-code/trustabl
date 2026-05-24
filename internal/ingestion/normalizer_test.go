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

func TestDetectSDKDeps_TSClaudeSDKFromPackageJSON(t *testing.T) {
	dir := t.TempDir()
	pkg := `{
  "name": "demo",
  "dependencies": {
    "@anthropic-ai/claude-agent-sdk": "^1.0.0"
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := detectSDKDeps(dir)
	var found bool
	for _, d := range deps {
		if d.Name == "claude-agent-sdk" && d.Source == "package.json" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected claude-agent-sdk@package.json in deps, got %+v", deps)
	}
}

func TestDetectSDKDeps_TSNeedleScopedToPackageJSONOnly(t *testing.T) {
	dir := t.TempDir()
	// A package.json with the TS package in devDependencies (a common pattern
	// for test code). The TS needle should find it. Combined with the first test,
	// this ensures the needle is scoped correctly: it fires on package.json but
	// not on Python manifests.
	pkg := `{
  "name": "test-suite",
  "devDependencies": {
    "@anthropic-ai/claude-agent-sdk": "^1.0.0"
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := detectSDKDeps(dir)
	var found bool
	for _, d := range deps {
		if d.Name == "claude-agent-sdk" && d.Source == "package.json" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected claude-agent-sdk@package.json even in devDependencies, got %+v", deps)
	}
}

// TestDetectSDKDeps_DoesNotCrossFireTSNeedleIntoPyprojectToml guards the
// substring-collision footgun: the TS needle "@anthropic-ai/claude-agent-sdk"
// contains the Python needle "claude-agent-sdk" as a substring. The TS needle
// is restricted to package.json, so a pyproject.toml that happens to mention
// the TS package id in prose MUST NOT produce a TS-source SDKDep. The Python
// needle WILL match (expected), but the TS needle should not. If this test
// ever starts failing, someone likely added package.json to the Python
// needle's Manifests list — read the maintainer comment in detectSDKDeps.
func TestDetectSDKDeps_DoesNotCrossFireTSNeedleIntoPyprojectToml(t *testing.T) {
	dir := t.TempDir()
	pyproject := `[project]
description = "tools for @anthropic-ai/claude-agent-sdk integration"
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := detectSDKDeps(dir)
	// The Python needle will match (expected behavior), but we must verify
	// the TS needle doesn't also produce a package.json-sourced entry.
	for _, d := range deps {
		if d.Source == "package.json" && d.Name == "claude-agent-sdk" {
			t.Errorf("TS needle should not fire when pyproject.toml mentions the TS package; got %+v", d)
		}
	}
}
