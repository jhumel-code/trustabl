package analysis_test

import (
	"testing"

	"github.com/trustabl/trustabl/internal/analysis"
	"github.com/trustabl/trustabl/internal/models"
)

func TestDiscoverTSMCPServers_CreateSdkMcpServer(t *testing.T) {
	src := `
import { createSdkMcpServer } from "@anthropic-ai/claude-agent-sdk";
const srv = createSdkMcpServer({ name: "my-tools", version: "1.0.0" });
`
	pf := parseTSForTest(t, "src/a.ts", src)
	got := analysis.DiscoverTSMCPServers([]analysis.ParsedFile{pf}, nil)
	if len(got) != 1 {
		t.Fatalf("got %d, want 1: %+v", len(got), got)
	}
	m := got[0]
	if m.Class != "createSdkMcpServer" {
		t.Errorf("Class: got %q want createSdkMcpServer", m.Class)
	}
	if m.Transport != "sdk" {
		t.Errorf("Transport: got %q want sdk", m.Transport)
	}
	if m.SDK != models.SDKClaudeAgentSDK || m.Language != models.LanguageTypeScript {
		t.Errorf("SDK/Language wrong: %+v", m)
	}
}
