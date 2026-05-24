package analysis_test

import (
	"testing"

	"github.com/trustabl/trustabl/internal/analysis"
	"github.com/trustabl/trustabl/internal/models"
)

func TestDiscoverTSAgents_InlineInQuery(t *testing.T) {
	src := `
import { query } from "@anthropic-ai/claude-agent-sdk";

const q = query({
  prompt: "Analyze",
  options: {
    agents: {
      analyst: {
        description: "Data analyst",
        prompt: "Analyze data"
      }
    }
  }
});
`
	pf := parseTSForTest(t, "src/a.ts", src)
	agents := analysis.DiscoverTSAgents([]analysis.ParsedFile{pf}, nil)
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1: %+v", len(agents), agents)
	}
	a := agents[0]
	if a.Name != "analyst" {
		t.Errorf("Name: got %q want %q", a.Name, "analyst")
	}
	if a.Class != "AgentDefinition" {
		t.Errorf("Class: got %q want %q", a.Class, "AgentDefinition")
	}
	if a.SDK != models.SDKClaudeAgentSDK {
		t.Errorf("SDK: got %q want %q", a.SDK, models.SDKClaudeAgentSDK)
	}
	if a.Language != models.LanguageTypeScript {
		t.Errorf("Language: got %q want %q", a.Language, models.LanguageTypeScript)
	}
	if a.Kwargs == nil || a.Kwargs.Children["description"] == nil {
		t.Errorf("Kwargs.description missing: %+v", a.Kwargs)
	}
}
