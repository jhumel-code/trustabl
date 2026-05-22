package generation

import (
	"strings"
	"testing"

	"github.com/trustabl/trustabl/internal/models"
)

// The OSH-* rule pack moved to a closed-source companion, so no live scan
// produces OpenShell findings today. The field-mapping logic in buildPolicy
// is kept in code so the pack can be re-introduced without engine changes
// (ARCHITECTURE.md § Stage 6). These tests exercise that mapping directly
// with synthetic findings so it stays covered and correct.

func oshFinding(ruleID, tool string) models.Finding {
	return models.Finding{
		RuleID:   ruleID,
		Category: models.CategoryOpenShell,
		ToolName: tool,
	}
}

func TestBuildPolicyMapsOSHFindings(t *testing.T) {
	findings := []models.Finding{
		oshFinding("OSH-001", ""),          // global deny
		oshFinding("OSH-002", "run_cmd"),   // per-tool allowed commands
		oshFinding("OSH-003", "run_cmd"),   // per-tool write prefixes
		oshFinding("OSH-005", "fetch_url"), // per-tool allowed hosts
	}

	doc := buildPolicy(findings, "test")

	if len(doc.Spec.GlobalDeny) != 1 {
		t.Fatalf("GlobalDeny = %d entries, want 1", len(doc.Spec.GlobalDeny))
	}
	if doc.Spec.GlobalDeny[0].RuleID != "OSH-001" {
		t.Fatalf("GlobalDeny[0].RuleID = %q, want OSH-001", doc.Spec.GlobalDeny[0].RuleID)
	}

	runCmd, ok := doc.Spec.Tools["run_cmd"]
	if !ok {
		t.Fatal("run_cmd tool missing from policy")
	}
	if runCmd.Commands == nil {
		t.Fatal("OSH-002 did not populate Commands for run_cmd")
	}
	if runCmd.Filesystem == nil {
		t.Fatal("OSH-003 did not populate Filesystem for run_cmd")
	}

	fetch, ok := doc.Spec.Tools["fetch_url"]
	if !ok || fetch.Network == nil {
		t.Fatal("OSH-005 did not populate Network for fetch_url")
	}
}

func TestDedupeDeny(t *testing.T) {
	in := []policyDeny{
		{RuleID: "OSH-001", Reason: "shell=True forbidden"},
		{RuleID: "OSH-001", Reason: "shell=True forbidden"}, // exact dup
		{RuleID: "OSH-009", Reason: "another reason"},
		{RuleID: "OSH-001", Reason: "different reason"}, // same ID, different reason: kept
	}
	out := dedupeDeny(in)

	if len(out) != 3 {
		t.Fatalf("dedupeDeny kept %d entries, want 3: %+v", len(out), out)
	}
	// Output is sorted by RuleID for determinism.
	for i := 1; i < len(out); i++ {
		if out[i-1].RuleID > out[i].RuleID {
			t.Fatalf("dedupeDeny output not sorted by RuleID: %+v", out)
		}
	}
}

func TestBuildPolicyDedupesRepeatGlobalDeny(t *testing.T) {
	// Two OSH-001 findings must collapse to a single GlobalDeny entry.
	findings := []models.Finding{oshFinding("OSH-001", ""), oshFinding("OSH-001", "")}
	doc := buildPolicy(findings, "test")
	if len(doc.Spec.GlobalDeny) != 1 {
		t.Fatalf("repeat OSH-001 produced %d GlobalDeny entries, want 1", len(doc.Spec.GlobalDeny))
	}
}

func TestGeneratePolicyFromOSHFindings(t *testing.T) {
	// Exercise the exported entry point on the findings path (not just the
	// defaults-only path the live pipeline currently takes).
	arts := GeneratePolicy([]models.Finding{oshFinding("OSH-001", "")}, "test")
	if len(arts) != 1 {
		t.Fatalf("GeneratePolicy returned %d artifacts, want 1", len(arts))
	}
	if arts[0].RelativePath != "openshell/policy.yaml" {
		t.Fatalf("artifact path = %q", arts[0].RelativePath)
	}
	if !strings.Contains(arts[0].Rationale, "1 OpenShell finding") {
		t.Fatalf("rationale = %q, want it to mention the finding count", arts[0].Rationale)
	}
}
