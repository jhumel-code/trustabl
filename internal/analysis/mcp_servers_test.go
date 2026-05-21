package analysis_test

import (
	"testing"

	"github.com/trustabl/trustabl/internal/analysis"
	"github.com/trustabl/trustabl/internal/models"
)

func TestMCPServers_InlineStdio(t *testing.T) {
	src := `
from agents import Agent
from agents.mcp import MCPServerStdio

agent = Agent(
    name="fs",
    mcp_servers=[MCPServerStdio(params={"command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem"]})],
)
`
	pf := parsePyFile(t, "main.py", src)
	inv := &models.RepoInventory{Agents: analysis.DiscoverAgents([]analysis.ParsedFile{pf})}
	analysis.ResolveEdges(inv, []analysis.ParsedFile{pf})

	if len(inv.MCPServers) != 1 {
		t.Fatalf("expected 1 MCP server, got %d", len(inv.MCPServers))
	}
	m := inv.MCPServers[0]
	if m.Class != "MCPServerStdio" {
		t.Errorf("Class = %v, want MCPServerStdio", m.Class)
	}
	if m.Transport != "stdio" {
		t.Errorf("Transport = %v, want stdio", m.Transport)
	}

	if len(inv.Agents[0].MCPServerRefs) != 1 || inv.Agents[0].MCPServerRefs[0].Resolved == nil {
		t.Errorf("expected 1 resolved MCP server ref, got %+v", inv.Agents[0].MCPServerRefs)
	}
}

func TestMCPServers_TransportDerivation(t *testing.T) {
	cases := []struct {
		class, transport string
	}{
		{"MCPServerStdio", "stdio"},
		{"MCPServerSse", "sse"},
		{"MCPServerStreamableHttp", "streamable_http"},
	}
	for _, tc := range cases {
		t.Run(tc.class, func(t *testing.T) {
			if got := analysis.MCPTransportFromClass(tc.class); got != tc.transport {
				t.Errorf("MCPTransportFromClass(%q) = %q, want %q", tc.class, got, tc.transport)
			}
		})
	}
}

func TestMCPServers_UnknownClassNotEmitted(t *testing.T) {
	src := `
from agents import Agent
agent = Agent(name="x", mcp_servers=[SomethingElse()])
`
	pf := parsePyFile(t, "main.py", src)
	inv := &models.RepoInventory{Agents: analysis.DiscoverAgents([]analysis.ParsedFile{pf})}
	analysis.ResolveEdges(inv, []analysis.ParsedFile{pf})
	if len(inv.MCPServers) != 0 {
		t.Errorf("expected zero MCP servers, got %+v", inv.MCPServers)
	}
	if len(inv.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(inv.Agents))
	}
	if len(inv.Agents[0].MCPServerRefs) != 1 {
		t.Fatalf("expected 1 MCPServerRef (count-preserving fallthrough), got %d", len(inv.Agents[0].MCPServerRefs))
	}
	if !inv.Agents[0].MCPServerRefs[0].External {
		t.Errorf("expected External=true for unrecognized class ref so Task 4 alias resolution can find it")
	}
}
