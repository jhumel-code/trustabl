package rules

import (
	"testing"

	"github.com/trustabl/trustabl/internal/models"
)

func TestAgentRuleDetector_LanguageGate_RejectsTSAgentForPythonRule(t *testing.T) {
	d := agentRuleDetector{rule: RuleDef{
		ID:        "TEST-101",
		Language:  models.LanguagePython,
		AppliesTo: []string{"claude_agent_definition"},
	}}
	tsAgent := models.AgentDef{
		SDK:      models.SDKClaudeAgentSDK,
		Class:    "AgentDefinition",
		Language: models.LanguageTypeScript,
	}
	if d.Applies(tsAgent) {
		t.Fatal("expected Applies()=false for TS agent vs Python rule, got true")
	}
}

func TestAgentRuleDetector_LanguageGate_AcceptsMatchingLanguage(t *testing.T) {
	d := agentRuleDetector{rule: RuleDef{
		ID:        "TEST-101",
		Language:  models.LanguagePython,
		AppliesTo: []string{"claude_agent_definition"},
	}}
	pyAgent := models.AgentDef{
		SDK:      models.SDKClaudeAgentSDK,
		Class:    "AgentDefinition",
		Language: models.LanguagePython,
	}
	if !d.Applies(pyAgent) {
		t.Fatal("expected Applies()=true for matching language, got false")
	}
}
