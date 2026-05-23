package sarif

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/trustabl/trustabl/internal/models"
)

// levelForSeverity maps Trustabl's 5-bucket severity to SARIF's 4-bucket level.
// Mapping locked in design doc D3. critical/high → error; medium → warning;
// low/info → note.
func levelForSeverity(s models.Severity) string {
	switch s {
	case models.SeverityCritical, models.SeverityHigh:
		return "error"
	case models.SeverityMedium:
		return "warning"
	default: // low, info, unknown
		return "note"
	}
}

// securitySeverityForSeverity maps to the GitHub "security-severity" 0–10
// string used to drive the Critical/High/Medium/Low badge bucketing. Cutoffs
// in the GitHub UI: ≥9 critical, ≥7 high, ≥4 medium, <4 low. Mapping locked
// in design doc D3.
func securitySeverityForSeverity(s models.Severity) string {
	switch s {
	case models.SeverityCritical:
		return "9.0"
	case models.SeverityHigh:
		return "7.5"
	case models.SeverityMedium:
		return "5.5"
	case models.SeverityLow:
		return "3.0"
	default: // info, unknown
		return "0.5"
	}
}

// tagsForFinding builds the rule-descriptor tag set: category, scope (parsed
// from the rule ID prefix when applicable), and language. Language defaults
// to "python" because Trustabl's discovery is python-only today and the
// loader fills "" with "python".
func tagsForFinding(f models.Finding) []string {
	tags := []string{}
	if f.Category != "" {
		tags = append(tags, string(f.Category))
	}
	// Trustabl's rule IDs encode scope implicitly:
	//   CSDK-0xx / OAI-0xx → tool scope
	//   CSDK-1xx / OAI-1xx → agent scope
	//   OAI-2xx           → repo scope
	//   META-xxx          → no scope tag
	if scope := scopeFromRuleID(f.RuleID); scope != "" {
		tags = append(tags, scope)
	}
	tags = append(tags, "python") // language; revisit when multi-language discovery lands
	return tags
}

// scopeFromRuleID returns the rule's scope tag based on its numeric prefix, or
// "" for META rules (no scope).
func scopeFromRuleID(id string) string {
	// id format: "<PREFIX>-<NNN>" where PREFIX is CSDK/OAI/META.
	// Scope buckets: 0xx tool, 1xx agent, 2xx repo.
	dash := -1
	for i := 0; i < len(id); i++ {
		if id[i] == '-' {
			dash = i
			break
		}
	}
	if dash < 0 || dash+1 >= len(id) {
		return ""
	}
	prefix := id[:dash]
	if prefix == "META" {
		return ""
	}
	first := id[dash+1]
	switch first {
	case '0':
		return "tool"
	case '1':
		return "agent"
	case '2':
		return "repo"
	}
	return ""
}

// ruleFromFinding builds a SARIF reportingDescriptor (rule catalog entry) from
// the first Finding emitted for a given rule. Title/Explanation/Fix are
// rule-stable; severity is rule-stable; confidence is rule-stable today (may
// vary per-finding once value-aware predicates land — the descriptor will then
// reflect the rule's default and result.rank carries the per-finding value).
func ruleFromFinding(f models.Finding) ReportingDescriptor {
	return ReportingDescriptor{
		ID:               f.RuleID,
		ShortDescription: &Message{Text: f.Title},
		FullDescription:  &Message{Text: f.Explanation},
		Help:             &Message{Text: f.SuggestedFix},
		DefaultConfiguration: &ReportingConfiguration{
			Level: levelForSeverity(f.Severity),
		},
		Properties: map[string]any{
			"security-severity": securitySeverityForSeverity(f.Severity),
			"confidence":        f.Confidence,
			"tags":              tagsForFinding(f),
		},
	}
}

// resultFromFinding builds a SARIF Result from a Finding. ruleIndex points at
// the entry for f.RuleID in tool.driver.rules (or nil if unknown — defensive,
// shouldn't happen in normal flow).
func resultFromFinding(f models.Finding, ruleIndex *int) Result {
	r := Result{
		RuleID:    f.RuleID,
		RuleIndex: ruleIndex,
		Message:   Message{Text: f.Explanation},
		Properties: map[string]any{
			"confidence": f.Confidence,
		},
		PartialFingerprints: map[string]string{
			"primaryLocationLineHash": fingerprintFor(f),
		},
	}
	rank := f.Confidence * 100
	r.Rank = &rank

	if f.SuggestedFix != "" {
		r.Fixes = []Fix{{Description: Message{Text: f.SuggestedFix}}}
	}

	// Locations: physical when we have a file; logical when we have a tool name.
	if f.FilePath != "" {
		phys := &PhysicalLocation{
			ArtifactLocation: ArtifactLocation{
				URI:       f.FilePath,
				URIBaseID: "REPO_ROOT",
			},
		}
		if f.Line > 0 {
			phys.Region = &Region{StartLine: f.Line}
		}
		loc := Location{PhysicalLocation: phys}
		if f.ToolName != "" {
			loc.LogicalLocations = []LogicalLocation{{Name: f.ToolName, Kind: "function"}}
		}
		r.Locations = []Location{loc}
	}

	// Kind: "informational" for META results and repo-scoped rule findings
	// (rule findings without a file location). Default (fail) for regular
	// located rule findings — emit empty so SARIF's default applies.
	if isInformational(f) {
		r.Kind = "informational"
	}

	return r
}

// isInformational returns true for findings that should carry SARIF
// kind="informational". Covers all META results that reach this builder
// (META-001/004 don't reach it — they're notifications) plus repo-scoped rule
// findings (which findingFromRule emits with empty FilePath/Line).
func isInformational(f models.Finding) bool {
	if len(f.RuleID) >= 4 && f.RuleID[:4] == "META" {
		return true
	}
	if f.FilePath == "" {
		return true
	}
	return false
}

// notificationFromFinding builds a SARIF Notification for META-001 / META-004.
// ruleIndex is the rule's position in tool.driver.rules; the notification
// references it via descriptor.index for consumer-side rule lookup.
func notificationFromFinding(f models.Finding, ruleIndex int) Notification {
	return Notification{
		Level:      "note",
		Message:    Message{Text: f.Explanation},
		Descriptor: &ReportingDescriptorReference{Index: ruleIndex},
		Properties: map[string]any{
			"rule_id": f.RuleID,
		},
	}
}

// fingerprintFor returns a stable hex SHA-256 of (RuleID|FilePath|ToolName).
// Identifies the same logical finding across scans even when line numbers
// shift, which is what GitHub Code Scanning uses to deduplicate alerts.
func fingerprintFor(f models.Finding) string {
	h := sha256.New()
	h.Write([]byte(f.RuleID))
	h.Write([]byte{'|'})
	h.Write([]byte(f.FilePath))
	h.Write([]byte{'|'})
	h.Write([]byte(f.ToolName))
	return hex.EncodeToString(h.Sum(nil))
}
