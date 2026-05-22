package review_test

import (
	"strings"
	"testing"

	"github.com/trustabl/trustabl/internal/models"
	"github.com/trustabl/trustabl/internal/review"
)

// TestRender_HostedToolsVisibleInHumanFormat is the regression test for the
// bug where ScanResult.HostedTools (populated by hosted-tool discovery) was
// surfaced in the JSON output but never rendered in the human format. A
// repo whose only tools are hosted (e.g. examples/research_bot using
// WebSearchTool) used to show "Tools found: 0" in human mode despite the
// JSON listing the tool.
func TestRender_HostedToolsVisibleInHumanFormat(t *testing.T) {
	result := models.ScanResult{
		Repo:      "./fixture",
		Languages: []models.Language{models.LanguagePython},
		SDKs:      []models.SDK{models.SDKOpenAIAgents},
		Agents: []models.AgentDef{
			{
				SDK:      models.SDKOpenAIAgents,
				Class:    "Agent",
				Name:     "search",
				FilePath: "agents/search.py",
				Line:     12,
				HostedToolRefs: []models.HostedToolRef{
					{Class: "WebSearchTool"},
				},
			},
		},
		HostedTools: []models.HostedToolDef{
			{Class: "WebSearchTool", SDK: models.SDKOpenAIAgents, FilePath: "agents/search.py", Line: 12},
		},
	}

	out := (&review.Renderer{NoColor: true}).Render(result)

	for _, want := range []string{
		"Hosted tools:   1",
		"WebSearchTool",
		"hosted tools:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q\n---\n%s", want, out)
		}
	}

	// "Tools found" must include the hosted tool class.
	if !strings.Contains(out, "Tools found:    1") {
		t.Errorf("Tools found count should be 1 (the hosted WebSearchTool); got:\n%s", out)
	}
}

func TestRender_MCPServersVisibleInHumanFormat(t *testing.T) {
	stdio := models.MCPServerDef{
		Class: "MCPServerStdio", Transport: "stdio", SDK: models.SDKOpenAIAgents,
		FilePath: "main.py", Line: 10,
	}
	result := models.ScanResult{
		Agents: []models.AgentDef{{
			SDK: models.SDKOpenAIAgents, Class: "Agent", Name: "fs",
			FilePath: "main.py", Line: 10,
			MCPServerRefs: []models.MCPServerRef{{Class: "MCPServerStdio", Resolved: &stdio}},
		}},
		MCPServers: []models.MCPServerDef{stdio},
	}

	out := (&review.Renderer{NoColor: true}).Render(result)
	for _, want := range []string{
		"MCP servers:    1",
		"MCPServerStdio (stdio)",
		"mcp servers:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q\n---\n%s", want, out)
		}
	}
}

func TestRender_SubagentsAndClaudeSettingsSections(t *testing.T) {
	result := models.ScanResult{
		Subagents: []models.SubagentDef{
			{Name: "researcher", FilePath: ".claude/agents/researcher.md",
				Tools: []string{"Read", "Glob", "Grep"}, Model: "haiku"},
		},
		ClaudeSettings: []models.ClaudeSettings{{
			FilePath:    ".claude/settings.json",
			DefaultMode: "acceptEdits",
			Permissions: models.ClaudePermissions{
				Allow: []models.PermissionRule{{Tool: "Bash", Pattern: "npm test", Raw: "Bash(npm test)"}},
				Deny:  []models.PermissionRule{{Tool: "WebFetch", Raw: "WebFetch"}},
			},
		}},
	}

	out := (&review.Renderer{NoColor: true}).Render(result)
	for _, want := range []string{
		"Subagents",                            // section header
		"researcher",                           // subagent name
		".claude/agents/researcher.md",         // subagent file path
		"tools: Read, Glob, Grep",              // tools list
		"model: haiku",                         // model
		"Claude settings",                      // section header
		".claude/settings.json",                // settings file path
		"defaultMode=acceptEdits",              // settings metadata
		"allow:1",                              // permission counts
		"deny:1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing %q\n---\n%s", want, out)
		}
	}
}

func TestRender_EmptyInventorySkipsNewLines(t *testing.T) {
	// Sanity: a repo with no hosted tools / MCP / subagents / settings must
	// not print the new summary lines. We don't want clutter on simple repos.
	result := models.ScanResult{
		Repo: "./fixture",
		Tools: []models.ToolDef{
			{Name: "do_thing", Kind: models.KindOpenAITool, Language: models.LanguagePython},
		},
	}
	out := (&review.Renderer{NoColor: true}).Render(result)
	for _, unwanted := range []string{
		"Hosted tools:",
		"MCP servers:",
		"Subagents:",
		"Claude settings:",
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("empty-inventory render leaked %q\n---\n%s", unwanted, out)
		}
	}
}
