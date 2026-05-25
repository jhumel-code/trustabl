package rules_test

import (
	"testing"

	"github.com/trustabl/trustabl/internal/models"
	"github.com/trustabl/trustabl/internal/rules"
)

func TestEvaluateSubagent_GrantsTool(t *testing.T) {
	expr := rules.MatchExpr{SubagentGrantsTool: []string{"Bash"}}
	grants := models.SubagentDef{Name: "x", Tools: []string{"Read", "Bash"}}
	noBash := models.SubagentDef{Name: "y", Tools: []string{"Read"}}
	inv := models.RepoInventory{}
	if !expr.EvaluateSubagent(grants, inv) {
		t.Errorf("expected match: subagent grants Bash")
	}
	if expr.EvaluateSubagent(noBash, inv) {
		t.Errorf("expected no match: subagent does not grant Bash")
	}
}

func TestEvaluateSubagent_EmptyExprVacuouslyTrue(t *testing.T) {
	var expr rules.MatchExpr
	if !expr.EvaluateSubagent(models.SubagentDef{Name: "x"}, models.RepoInventory{}) {
		t.Errorf("empty MatchExpr should match vacuously")
	}
}

func TestEvaluateSubagent_NotCombinator(t *testing.T) {
	inner := rules.MatchExpr{SubagentGrantsTool: []string{"Bash"}}
	expr := rules.MatchExpr{Not: &inner}
	noBash := models.SubagentDef{Name: "y", Tools: []string{"Read"}}
	if !expr.EvaluateSubagent(noBash, models.RepoInventory{}) {
		t.Errorf("not(grants Bash) should match a subagent without Bash")
	}
}
