package rules_test

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/trustabl/karenctl/internal/models"
	"github.com/trustabl/karenctl/internal/rules"
)

// TestScoreTableInSync verifies that the generated region of docs/scoring.md
// matches the scores computed from the embedded policy YAMLs.
//
// If this test fails, run:
//
//	go run ./tools/genscoretable
func TestScoreTableInSync(t *testing.T) {
	policies, err := rules.Load(rules.DefaultFS())
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	byCategory := map[string][]scoreRow{}
	for _, pf := range policies {
		cat := string(pf.Policy.Category)
		for _, r := range pf.Rules {
			byCategory[cat] = append(byCategory[cat], scoreRow{
				id:         r.ID,
				title:      r.Title,
				severity:   r.Severity,
				confidence: r.Confidence,
				score:      roundScore(models.BaseScoreWeight(r.Severity) * r.Confidence),
			})
		}
	}
	for cat := range byCategory {
		sort.Slice(byCategory[cat], func(i, j int) bool {
			return byCategory[cat][i].id < byCategory[cat][j].id
		})
	}

	want := generateScoreTable(byCategory)

	// Read docs/scoring.md — test working dir is the package dir, so ../../docs.
	raw, err := os.ReadFile("../../docs/scoring.md")
	if err != nil {
		t.Fatalf("read scoring.md: %v", err)
	}

	const begin = "<!-- score-table:begin -->"
	const end = "<!-- score-table:end -->"
	doc := string(raw)
	bi := strings.Index(doc, begin)
	ei := strings.Index(doc, end)
	if bi == -1 || ei == -1 {
		t.Fatal("score-table markers not found in docs/scoring.md")
	}
	got := strings.TrimPrefix(doc[bi+len(begin):ei], "\n")

	if got != want {
		t.Errorf("docs/scoring.md score table is out of sync with policy YAMLs.\n"+
			"Run: go run ./tools/genscoretable\n\n"+
			"First diff:\n%s", firstDiff(got, want))
	}
}

type scoreRow struct {
	id, title  string
	severity   models.Severity
	confidence float64
	score      float64
}

func roundScore(v float64) float64 {
	return math.Round(v*10) / 10
}

var scoreTableCategoryOrder = []string{
	"claude_sdk", "openshell", "openai_sdk", "mcp", "catalog",
}

var scoreTableCategoryLabel = map[string]string{
	"claude_sdk":  "Claude Agent SDK (CSDK)",
	"openshell":   "OpenShell (OSH)",
	"openai_sdk":  "OpenAI Agents SDK (OAIS)",
	"mcp":         "MCP (MCP)",
	"catalog":     "Catalog capability-class (CATL)",
}

func sevDisplay(s models.Severity) string {
	switch s {
	case models.SeverityCritical:
		return "Critical"
	case models.SeverityHigh:
		return "High"
	case models.SeverityMedium:
		return "Medium"
	case models.SeverityLow:
		return "Low"
	default:
		return string(s)
	}
}

func generateScoreTable(byCategory map[string][]scoreRow) string {
	var b strings.Builder

	b.WriteString("## All rules scored\n")

	for _, cat := range scoreTableCategoryOrder {
		rows, ok := byCategory[cat]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "\n### %s\n\n", scoreTableCategoryLabel[cat])
		b.WriteString("| ID | Title | Sev | Conf | Score |\n")
		b.WriteString("|----|-------|-----|------|-------|\n")
		for _, r := range rows {
			fmt.Fprintf(&b, "| %-8s | %-50s | %-8s | %.2f | %5.1f |\n",
				r.id, r.title, sevDisplay(r.severity), r.confidence, r.score)
		}
	}

	b.WriteString("\n---\n\n")
	b.WriteString(generateDistribution(byCategory))

	return b.String()
}

func generateDistribution(byCategory map[string][]scoreRow) string {
	bands := []struct {
		label string
		lo    float64
		hi    float64
	}{
		{"90–100", 90, 100},
		{"70– 89", 70, 89.9},
		{"40– 69", 40, 69.9},
		{"10– 39", 10, 39.9},
	}

	var all []scoreRow
	for _, cat := range scoreTableCategoryOrder {
		all = append(all, byCategory[cat]...)
	}

	var b strings.Builder
	b.WriteString("## Score distribution\n\n```\n")
	for _, band := range bands {
		var matches []string
		for _, r := range all {
			if r.score >= band.lo && r.score <= band.hi {
				matches = append(matches, fmt.Sprintf("%s (%.1f)", r.id, r.score))
			}
		}
		bar := strings.Repeat("█", min(len(matches)*3, 48))
		if len(matches) == 0 {
			fmt.Fprintf(&b, "%s   (none)\n", band.label)
			continue
		}
		line := strings.Join(matches, ", ")
		if len(line) <= 55 {
			fmt.Fprintf(&b, "%s   %s  %s\n", band.label, bar, line)
		} else {
			fmt.Fprintf(&b, "%s   %s\n", band.label, bar)
			for len(matches) > 0 {
				chunk := matches
				if len(chunk) > 3 {
					chunk = matches[:3]
				}
				fmt.Fprintf(&b, "           %s\n", strings.Join(chunk, ", "))
				matches = matches[len(chunk):]
			}
		}
	}
	b.WriteString("```\n")
	return b.String()
}

func firstDiff(got, want string) string {
	gl := strings.Split(got, "\n")
	wl := strings.Split(want, "\n")
	for i := 0; i < len(gl) && i < len(wl); i++ {
		if gl[i] != wl[i] {
			return fmt.Sprintf("line %d\n  got:  %q\n  want: %q", i+1, gl[i], wl[i])
		}
	}
	if len(gl) != len(wl) {
		return fmt.Sprintf("line count differs: got %d, want %d", len(gl), len(wl))
	}
	return "(no diff found)"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// firstDiff helper uses bytes.Compare for the fast path.
var _ = bytes.Compare
