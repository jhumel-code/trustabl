package sarif

import (
	"testing"

	"github.com/trustabl/trustabl/internal/models"
)

func TestLevelForSeverity(t *testing.T) {
	cases := map[models.Severity]string{
		models.SeverityCritical: "error",
		models.SeverityHigh:     "error",
		models.SeverityMedium:   "warning",
		models.SeverityLow:      "note",
		models.SeverityInfo:     "note",
	}
	for sev, want := range cases {
		if got := levelForSeverity(sev); got != want {
			t.Errorf("levelForSeverity(%q) = %q, want %q", sev, got, want)
		}
	}
}

func TestSecuritySeverityForSeverity(t *testing.T) {
	cases := map[models.Severity]string{
		models.SeverityCritical: "9.0",
		models.SeverityHigh:     "7.5",
		models.SeverityMedium:   "5.5",
		models.SeverityLow:      "3.0",
		models.SeverityInfo:     "0.5",
	}
	for sev, want := range cases {
		if got := securitySeverityForSeverity(sev); got != want {
			t.Errorf("securitySeverityForSeverity(%q) = %q, want %q", sev, got, want)
		}
	}
}

func TestTagsForFinding(t *testing.T) {
	// Category, Scope (derived from RuleID), Language ("python" default).
	f := models.Finding{
		RuleID:   "OAI-101",
		Category: models.CategoryOpenAISDK,
	}
	tags := tagsForFinding(f)
	wantContains := []string{"openai_sdk", "python"}
	for _, w := range wantContains {
		found := false
		for _, tag := range tags {
			if tag == w {
				found = true
			}
		}
		if !found {
			t.Errorf("tagsForFinding missing %q in %v", w, tags)
		}
	}
}

func TestRuleFromFinding(t *testing.T) {
	f := models.Finding{
		RuleID:       "OAI-101",
		Category:     models.CategoryOpenAISDK,
		Severity:     models.SeverityHigh,
		Title:        "Agent with shell tools and no input_guardrails",
		Explanation:  "An agent that exposes shell-invoking tools without input_guardrails is unsafe.",
		SuggestedFix: "Add input_guardrails = [...] to the Agent(...) constructor.",
		Confidence:   0.85,
	}
	rd := ruleFromFinding(f)
	if rd.ID != "OAI-101" {
		t.Errorf("ID = %q, want OAI-101", rd.ID)
	}
	if rd.ShortDescription == nil || rd.ShortDescription.Text != f.Title {
		t.Errorf("ShortDescription = %v, want %q", rd.ShortDescription, f.Title)
	}
	if rd.FullDescription == nil || rd.FullDescription.Text != f.Explanation {
		t.Errorf("FullDescription = %v, want %q", rd.FullDescription, f.Explanation)
	}
	if rd.Help == nil || rd.Help.Text != f.SuggestedFix {
		t.Errorf("Help = %v, want %q", rd.Help, f.SuggestedFix)
	}
	if rd.DefaultConfiguration == nil || rd.DefaultConfiguration.Level != "error" {
		t.Errorf("DefaultConfiguration.Level = %v, want error", rd.DefaultConfiguration)
	}
	if rd.Properties["security-severity"] != "7.5" {
		t.Errorf("security-severity = %v, want 7.5", rd.Properties["security-severity"])
	}
	if rd.Properties["confidence"] != 0.85 {
		t.Errorf("confidence = %v, want 0.85", rd.Properties["confidence"])
	}
	tags, ok := rd.Properties["tags"].([]string)
	if !ok {
		t.Fatalf("tags missing or wrong type: %T", rd.Properties["tags"])
	}
	if len(tags) == 0 {
		t.Error("tags is empty")
	}
}

func TestResultFromFinding_LocatedToolFinding(t *testing.T) {
	f := models.Finding{
		RuleID:       "OAI-005",
		Category:     models.CategoryOpenAISDK,
		Severity:     models.SeverityHigh,
		ToolName:     "fetch_url",
		FilePath:     "agents/web.py",
		Line:         42,
		Title:        "Network call has no timeout",
		Explanation:  "An HTTP call without timeout can hang.",
		SuggestedFix: "Pass timeout=5 to the request.",
		Confidence:   0.85,
	}
	idx := 3
	r := resultFromFinding(f, &idx)

	if r.RuleID != "OAI-005" {
		t.Errorf("RuleID = %q", r.RuleID)
	}
	if r.RuleIndex == nil || *r.RuleIndex != 3 {
		t.Errorf("RuleIndex = %v, want 3", r.RuleIndex)
	}
	if r.Kind != "" {
		t.Errorf("Kind = %q, want \"\" (default fail)", r.Kind)
	}
	if r.Message.Text != f.Explanation {
		t.Errorf("Message.Text = %q", r.Message.Text)
	}
	if len(r.Locations) != 1 {
		t.Fatalf("Locations len = %d, want 1", len(r.Locations))
	}
	loc := r.Locations[0]
	if loc.PhysicalLocation == nil {
		t.Fatal("PhysicalLocation nil")
	}
	if loc.PhysicalLocation.ArtifactLocation.URI != "agents/web.py" {
		t.Errorf("URI = %q", loc.PhysicalLocation.ArtifactLocation.URI)
	}
	if loc.PhysicalLocation.ArtifactLocation.URIBaseID != "REPO_ROOT" {
		t.Errorf("URIBaseID = %q", loc.PhysicalLocation.ArtifactLocation.URIBaseID)
	}
	if loc.PhysicalLocation.Region == nil || loc.PhysicalLocation.Region.StartLine != 42 {
		t.Errorf("StartLine wrong: %+v", loc.PhysicalLocation.Region)
	}
	if len(loc.LogicalLocations) != 1 || loc.LogicalLocations[0].Name != "fetch_url" {
		t.Errorf("LogicalLocations = %+v", loc.LogicalLocations)
	}
	if loc.LogicalLocations[0].Kind != "function" {
		t.Errorf("LogicalLocation Kind = %q", loc.LogicalLocations[0].Kind)
	}
	if len(r.Fixes) != 1 || r.Fixes[0].Description.Text != f.SuggestedFix {
		t.Errorf("Fixes = %+v", r.Fixes)
	}
	if r.Rank == nil || *r.Rank != 85.0 {
		t.Errorf("Rank = %v, want 85.0", r.Rank)
	}
	if r.Properties["confidence"] != 0.85 {
		t.Errorf("confidence prop = %v", r.Properties["confidence"])
	}
	if r.PartialFingerprints["primaryLocationLineHash"] == "" {
		t.Error("PartialFingerprints.primaryLocationLineHash is empty")
	}
}

func TestResultFromFinding_RepoScopedFindingNoLocation(t *testing.T) {
	// Repo-scoped rule findings come out of findingFromRule with FilePath=""
	// and Line=0. Per D5: emit as kind="informational", omit locations.
	f := models.Finding{
		RuleID:      "OAI-201",
		Severity:    models.SeverityMedium,
		Title:       "OpenAI Agents SDK present but no custom tracing",
		Explanation: "Tracing is enabled by default but no custom processor is configured.",
		Confidence:  0.7,
	}
	r := resultFromFinding(f, nil)
	if r.Kind != "informational" {
		t.Errorf("Kind = %q, want informational", r.Kind)
	}
	if len(r.Locations) != 0 {
		t.Errorf("Locations should be empty, got %d", len(r.Locations))
	}
}

func TestResultFromFinding_META002LocatedAtManifest(t *testing.T) {
	// Per D4: META-002 emits as a result with a location pointing at the dep
	// manifest (FilePath on the Finding). policy_selection.go enhancement
	// (Task 6) sets FilePath = dep.Source.
	f := models.Finding{
		RuleID:      "META-002",
		Severity:    models.SeverityInfo,
		FilePath:    "pyproject.toml",
		Title:       "Declared SDK dependency has no observed code use",
		Explanation: "The 'openai-agents' dep is declared but not used in any source file.",
		Confidence:  1.0,
	}
	r := resultFromFinding(f, nil)
	if r.Kind != "informational" {
		t.Errorf("Kind = %q, want informational", r.Kind)
	}
	if len(r.Locations) != 1 || r.Locations[0].PhysicalLocation.ArtifactLocation.URI != "pyproject.toml" {
		t.Errorf("META-002 should attribute to manifest path, got %+v", r.Locations)
	}
	if len(r.Locations[0].LogicalLocations) != 0 {
		t.Errorf("META-002 has no ToolName so no logicalLocations expected")
	}
}

func TestNotificationFromFinding(t *testing.T) {
	// Per D4: META-001 / META-004 emit as notifications.
	f := models.Finding{
		RuleID:      "META-001",
		Severity:    models.SeverityInfo,
		Title:       "Unaudited SDK in use",
		Explanation: "This repo uses SDK \"google_adk\", which Trustabl does not currently audit.",
		Confidence:  1.0,
	}
	n := notificationFromFinding(f, 7)
	if n.Level != "note" {
		t.Errorf("Level = %q, want note", n.Level)
	}
	if n.Message.Text != f.Explanation {
		t.Errorf("Message = %q", n.Message.Text)
	}
	if n.Descriptor == nil || n.Descriptor.Index != 7 {
		t.Errorf("Descriptor = %+v", n.Descriptor)
	}
	if n.Properties["rule_id"] != "META-001" {
		t.Errorf("properties.rule_id = %v", n.Properties["rule_id"])
	}
}
