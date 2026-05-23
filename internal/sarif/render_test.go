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
