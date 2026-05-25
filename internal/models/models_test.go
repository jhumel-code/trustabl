package models_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/trustabl/trustabl/internal/models"
)

func TestValidScope_Subagent(t *testing.T) {
	if !models.ValidScope(models.ScopeSubagent) {
		t.Errorf("ScopeSubagent should be valid")
	}
	if models.ScopeSubagent != "subagent" {
		t.Errorf("ScopeSubagent: got %q, want \"subagent\"", models.ScopeSubagent)
	}
}

func TestScanResult_RulesProvenanceFieldsSerialize(t *testing.T) {
	r := models.ScanResult{
		RulesSource:    "https://example.com/rules",
		RulesVersion:   "abc123",
		RulesFromCache: true,
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`"rules_source":"https://example.com/rules"`,
		`"rules_version":"abc123"`,
		`"rules_from_cache":true`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("JSON missing %s\ngot: %s", want, b)
		}
	}
}
