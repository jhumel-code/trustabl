package analysis_test

import (
	"testing"

	"github.com/trustabl/trustabl/internal/analysis"
	"github.com/trustabl/trustabl/internal/models"
)

func TestHostedTools_WebSearchTool(t *testing.T) {
	src := `
from agents import Agent, WebSearchTool

agent = Agent(name="search", tools=[WebSearchTool()])
`
	pf := parsePyFile(t, "main.py", src)
	inv := &models.RepoInventory{
		Agents: analysis.DiscoverAgents([]analysis.ParsedFile{pf}),
	}
	analysis.ResolveEdges(inv, []analysis.ParsedFile{pf})

	if len(inv.HostedTools) != 1 {
		t.Fatalf("expected 1 hosted tool, got %d", len(inv.HostedTools))
	}
	h := inv.HostedTools[0]
	if h.Class != "WebSearchTool" {
		t.Errorf("Class = %v, want WebSearchTool", h.Class)
	}
	if h.SDK != models.SDKOpenAIAgents {
		t.Errorf("SDK = %v, want openai_agents", h.SDK)
	}

	if len(inv.Agents) != 1 || len(inv.Agents[0].HostedToolRefs) != 1 {
		t.Fatalf("expected 1 hosted tool ref on agent, got %+v", inv.Agents)
	}
	ref := inv.Agents[0].HostedToolRefs[0]
	if ref.Resolved == nil || ref.Resolved.Class != "WebSearchTool" {
		t.Errorf("ref not resolved: %+v", ref)
	}
}

func TestHostedTools_AllKnownClasses(t *testing.T) {
	classes := []string{
		"WebSearchTool", "FileSearchTool", "ComputerTool", "HostedMCPTool",
		"CodeInterpreterTool", "ImageGenerationTool", "LocalShellTool",
		"ShellTool", "ApplyPatchTool", "CustomTool", "ToolSearchTool",
	}
	for _, c := range classes {
		t.Run(c, func(t *testing.T) {
			src := "from agents import Agent\nagent = Agent(name=\"x\", tools=[" + c + "()])"
			pf := parsePyFile(t, "main.py", src)
			inv := &models.RepoInventory{Agents: analysis.DiscoverAgents([]analysis.ParsedFile{pf})}
			analysis.ResolveEdges(inv, []analysis.ParsedFile{pf})
			if len(inv.HostedTools) != 1 || inv.HostedTools[0].Class != c {
				t.Errorf("class %s: expected exactly one HostedTool with that class, got %+v", c, inv.HostedTools)
			}
		})
	}
}

func TestHostedTools_UnknownClassIgnored(t *testing.T) {
	src := `
from agents import Agent

agent = Agent(name="x", tools=[NotAHostedTool()])
`
	pf := parsePyFile(t, "main.py", src)
	inv := &models.RepoInventory{Agents: analysis.DiscoverAgents([]analysis.ParsedFile{pf})}
	analysis.ResolveEdges(inv, []analysis.ParsedFile{pf})
	if len(inv.HostedTools) != 0 {
		t.Errorf("expected zero hosted tools, got %+v", inv.HostedTools)
	}
}

func TestHostedTools_DeterministicOrder(t *testing.T) {
	src := `
from agents import Agent, WebSearchTool, FileSearchTool

a = Agent(name="a", tools=[WebSearchTool(), FileSearchTool(vector_store_ids=["v"])])
b = Agent(name="b", tools=[FileSearchTool(vector_store_ids=["v"]), WebSearchTool()])
`
	pf := parsePyFile(t, "main.py", src)
	inv := &models.RepoInventory{Agents: analysis.DiscoverAgents([]analysis.ParsedFile{pf})}
	analysis.ResolveEdges(inv, []analysis.ParsedFile{pf})
	if len(inv.HostedTools) != 4 {
		t.Fatalf("expected 4 hosted tools, got %d", len(inv.HostedTools))
	}
	// Sorted by (FilePath, Line, Class) — see hosted_tools.go.
	for i := 1; i < len(inv.HostedTools); i++ {
		prev, curr := inv.HostedTools[i-1], inv.HostedTools[i]
		if prev.FilePath > curr.FilePath ||
			(prev.FilePath == curr.FilePath && prev.Line > curr.Line) ||
			(prev.FilePath == curr.FilePath && prev.Line == curr.Line && prev.Class > curr.Class) {
			t.Errorf("HostedTools not sorted at index %d: %+v then %+v", i, prev, curr)
		}
	}
}
