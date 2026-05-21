package analysis_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/trustabl/trustabl/internal/analysis"
	"github.com/trustabl/trustabl/internal/models"
)

func writeFixture(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSubagents_ParsesFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, ".claude/agents/inbox-searcher.md", `---
name: inbox-searcher
description: "Email search specialist."
tools: Read, Bash, Glob, Grep, mcp__email__search_inbox
---

# Body content
`)
	manifest := models.ScanManifest{
		RepoRoot: dir,
		Components: []models.AgentComponent{
			{Kind: models.ComponentSubagent, Path: ".claude/agents/inbox-searcher.md"},
		},
	}
	got := analysis.DiscoverSubagents(manifest)
	if len(got) != 1 {
		t.Fatalf("expected 1 subagent, got %d", len(got))
	}
	want := models.SubagentDef{
		Name:        "inbox-searcher",
		Description: "Email search specialist.",
		Tools:       []string{"Read", "Bash", "Glob", "Grep", "mcp__email__search_inbox"},
		FilePath:    ".claude/agents/inbox-searcher.md",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("got  %+v\nwant %+v", got[0], want)
	}
}

func TestSubagents_NoFrontmatterReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, ".claude/agents/x.md", "Just a body, no frontmatter.\n")
	manifest := models.ScanManifest{
		RepoRoot: dir,
		Components: []models.AgentComponent{
			{Kind: models.ComponentSubagent, Path: ".claude/agents/x.md"},
		},
	}
	if got := analysis.DiscoverSubagents(manifest); len(got) != 0 {
		t.Errorf("expected zero subagents from body-only file, got %+v", got)
	}
}

func TestSubagents_DeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, ".claude/agents/b.md", "---\nname: b\ndescription: B\n---\n")
	writeFixture(t, dir, ".claude/agents/a.md", "---\nname: a\ndescription: A\n---\n")
	manifest := models.ScanManifest{
		RepoRoot: dir,
		Components: []models.AgentComponent{
			{Kind: models.ComponentSubagent, Path: ".claude/agents/b.md"},
			{Kind: models.ComponentSubagent, Path: ".claude/agents/a.md"},
		},
	}
	got := analysis.DiscoverSubagents(manifest)
	if len(got) != 2 || got[0].FilePath > got[1].FilePath {
		t.Errorf("expected sorted by FilePath, got %+v", got)
	}
}

func TestSubagents_ModelField(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, ".claude/agents/r.md", "---\nname: r\ndescription: R\ntools: Read\nmodel: haiku\n---\n")
	manifest := models.ScanManifest{
		RepoRoot: dir,
		Components: []models.AgentComponent{
			{Kind: models.ComponentSubagent, Path: ".claude/agents/r.md"},
		},
	}
	got := analysis.DiscoverSubagents(manifest)
	if len(got) != 1 || got[0].Model != "haiku" {
		t.Errorf("expected model=haiku, got %+v", got)
	}
}
