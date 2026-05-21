package analysis_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/trustabl/trustabl/internal/analysis"
	"github.com/trustabl/trustabl/internal/models"
)

func TestClaudeSettings_ParsesPermissions(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, ".claude/settings.json", `{
		"permissions": {
			"allow": ["Bash(npm run *)", "Read(./.env)"],
			"deny":  ["Bash(curl *)", "WebFetch"],
			"ask":   ["Edit(./src/**)"],
			"defaultMode": "acceptEdits",
			"additionalDirectories": ["../docs/"]
		},
		"env": {"FOO": "bar"},
		"hooks": {"PreSessionStart": {"type": "command", "command": "./hooks/pre.sh"}}
	}`)

	manifest := models.ScanManifest{
		RepoRoot: dir,
		Components: []models.AgentComponent{
			{Kind: models.ComponentClaudeSettings, Path: ".claude/settings.json"},
		},
	}
	got := analysis.DiscoverClaudeSettings(manifest)
	if len(got) != 1 {
		t.Fatalf("expected 1 settings record, got %d", len(got))
	}
	s := got[0]
	if filepath.ToSlash(s.FilePath) != ".claude/settings.json" {
		t.Errorf("FilePath = %v", s.FilePath)
	}
	if s.DefaultMode != "acceptEdits" {
		t.Errorf("DefaultMode = %v", s.DefaultMode)
	}
	wantAllow := []models.PermissionRule{
		{Tool: "Bash", Pattern: "npm run *", Raw: "Bash(npm run *)"},
		{Tool: "Read", Pattern: "./.env", Raw: "Read(./.env)"},
	}
	if !reflect.DeepEqual(s.Permissions.Allow, wantAllow) {
		t.Errorf("Allow = %+v\nwant %+v", s.Permissions.Allow, wantAllow)
	}
	wantDeny := []models.PermissionRule{
		{Tool: "Bash", Pattern: "curl *", Raw: "Bash(curl *)"},
		{Tool: "WebFetch", Raw: "WebFetch"},
	}
	if !reflect.DeepEqual(s.Permissions.Deny, wantDeny) {
		t.Errorf("Deny = %+v\nwant %+v", s.Permissions.Deny, wantDeny)
	}
	if !s.HasEnvBlock || !s.HasHooks {
		t.Errorf("HasEnvBlock=%v HasHooks=%v", s.HasEnvBlock, s.HasHooks)
	}
	if !reflect.DeepEqual(s.AdditionalDirs, []string{"../docs/"}) {
		t.Errorf("AdditionalDirs = %v", s.AdditionalDirs)
	}
}

func TestParsePermissionRule_Grammar(t *testing.T) {
	cases := []struct {
		raw, tool, pattern string
	}{
		{"Bash", "Bash", ""},
		{"Bash(npm install)", "Bash", "npm install"},
		{"Read(./secrets/**)", "Read", "./secrets/**"},
		{"WebFetch(domain:example.com)", "WebFetch", "domain:example.com"},
		{"MCP(server:github)", "MCP", "server:github"},
		{"mcp__github__list_issues", "MCP", "github__list_issues"},
		{"Agent(researcher)", "Agent", "researcher"},
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			got := analysis.ParsePermissionRule(c.raw)
			if got.Tool != c.tool || got.Pattern != c.pattern || got.Raw != c.raw {
				t.Errorf("ParsePermissionRule(%q) = %+v, want tool=%q pattern=%q",
					c.raw, got, c.tool, c.pattern)
			}
		})
	}
}

func TestClaudeSettings_MalformedJSONSkipped(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, ".claude/settings.json", `{not json`)
	manifest := models.ScanManifest{
		RepoRoot: dir,
		Components: []models.AgentComponent{
			{Kind: models.ComponentClaudeSettings, Path: ".claude/settings.json"},
		},
	}
	if got := analysis.DiscoverClaudeSettings(manifest); len(got) != 0 {
		t.Errorf("expected zero settings from malformed JSON, got %+v", got)
	}
}
