package analysis_test

import (
	"reflect"
	"testing"

	"github.com/trustabl/trustabl/internal/analysis"
)

func TestClaudeAccessors_BuiltinTools(t *testing.T) {
	src := `
from claude_agent_sdk import AgentDefinition
agent = AgentDefinition(
    description="d", prompt="p",
    tools=["Bash", "Read", "Edit"],
    disallowedTools=["WebFetch"],
    permissionMode="acceptEdits",
    mcpServers=["github"],
)
`
	pf := parsePyFile(t, "main.py", src)
	agents := analysis.DiscoverAgents([]analysis.ParsedFile{pf})
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent")
	}
	a := agents[0]

	if got := analysis.ClaudeBuiltinTools(&a); !reflect.DeepEqual(got, []string{"Bash", "Read", "Edit"}) {
		t.Errorf("ClaudeBuiltinTools = %v", got)
	}
	if got := analysis.ClaudeDisallowedTools(&a); !reflect.DeepEqual(got, []string{"WebFetch"}) {
		t.Errorf("ClaudeDisallowedTools = %v", got)
	}
	if got := analysis.ClaudePermissionMode(&a); got != "acceptEdits" {
		t.Errorf("ClaudePermissionMode = %v", got)
	}
	if got := analysis.ClaudeMCPServers(&a); !reflect.DeepEqual(got, []string{"github"}) {
		t.Errorf("ClaudeMCPServers = %v", got)
	}
}

func TestClaudeAccessors_MissingKwargs(t *testing.T) {
	src := `
from claude_agent_sdk import AgentDefinition
agent = AgentDefinition(description="d", prompt="p")
`
	pf := parsePyFile(t, "main.py", src)
	a := analysis.DiscoverAgents([]analysis.ParsedFile{pf})[0]

	if got := analysis.ClaudeBuiltinTools(&a); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	if got := analysis.ClaudePermissionMode(&a); got != "" {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestClaudeAccessors_IgnoresNonClaudeAgents(t *testing.T) {
	src := `
from agents import Agent
agent = Agent(name="x", tools=[])
`
	pf := parsePyFile(t, "main.py", src)
	a := analysis.DiscoverAgents([]analysis.ParsedFile{pf})[0]

	if got := analysis.ClaudeBuiltinTools(&a); got != nil {
		t.Errorf("OpenAI agent should return nil from Claude accessor, got %v", got)
	}
}
