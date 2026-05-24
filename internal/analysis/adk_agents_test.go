package analysis_test

import (
	"testing"

	"github.com/trustabl/trustabl/internal/analysis"
	"github.com/trustabl/trustabl/internal/models"
)

func TestDiscoverADKAgents_LlmAgentMinimal(t *testing.T) {
	src := `from google.adk.agents import LlmAgent

root = LlmAgent(
    name="root",
    model="gemini-2.5-flash",
    instruction="Be helpful.",
)
`
	pf := parsePyFile(t, "main.py", src)
	agents := analysis.DiscoverADKAgents([]analysis.ParsedFile{pf})
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	a := agents[0]
	if a.SDK != models.SDKGoogleADK {
		t.Errorf("SDK: got %q, want %q", a.SDK, models.SDKGoogleADK)
	}
	if a.Class != "LlmAgent" {
		t.Errorf("Class: got %q, want %q", a.Class, "LlmAgent")
	}
	if a.Name != "root" {
		t.Errorf("Name: got %q, want %q", a.Name, "root")
	}
	if a.FilePath != "main.py" {
		t.Errorf("FilePath: got %q, want %q", a.FilePath, "main.py")
	}
}

func TestDiscoverADKAgents_AllClasses(t *testing.T) {
	src := `from google.adk.agents import LlmAgent, Agent, SequentialAgent, ParallelAgent, LoopAgent, LanggraphAgent

a = LlmAgent(name="a")
b = Agent(name="b")
c = SequentialAgent(name="c", sub_agents=[a])
d = ParallelAgent(name="d", sub_agents=[a, b])
e = LoopAgent(name="e", sub_agents=[a])
f = LanggraphAgent(name="f")
`
	pf := parsePyFile(t, "main.py", src)
	agents := analysis.DiscoverADKAgents([]analysis.ParsedFile{pf})
	if len(agents) != 6 {
		t.Fatalf("got %d agents, want 6", len(agents))
	}
	wantByName := map[string]string{
		"a": "LlmAgent",
		"b": "LlmAgent", // alias normalization
		"c": "SequentialAgent",
		"d": "ParallelAgent",
		"e": "LoopAgent",
		"f": "LanggraphAgent",
	}
	for _, a := range agents {
		wantClass, ok := wantByName[a.Name]
		if !ok {
			t.Errorf("unexpected agent name %q", a.Name)
			continue
		}
		if a.Class != wantClass {
			t.Errorf("agent %q: Class = %q, want %q", a.Name, a.Class, wantClass)
		}
		if a.SDK != models.SDKGoogleADK {
			t.Errorf("agent %q: SDK = %q, want google_adk", a.Name, a.SDK)
		}
	}
}

func TestDiscoverADKAgents_ImportGate(t *testing.T) {
	// Agent() in a file with no google.adk import must NOT be classified as ADK.
	src := `from agents import Agent

a = Agent(name="oai_agent")
`
	pf := parsePyFile(t, "main.py", src)
	agents := analysis.DiscoverADKAgents([]analysis.ParsedFile{pf})
	if len(agents) != 0 {
		t.Errorf("got %d agents from OpenAI-import file, want 0", len(agents))
	}
}

func TestDiscoverADKAgents_OpaqueOnSplat(t *testing.T) {
	src := `from google.adk.agents import LlmAgent

cfg = {"name": "a"}
a = LlmAgent(**cfg)
`
	pf := parsePyFile(t, "main.py", src)
	agents := analysis.DiscoverADKAgents([]analysis.ParsedFile{pf})
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	if !agents[0].Opaque {
		t.Errorf("Opaque: got false, want true (LlmAgent(**cfg))")
	}
}
