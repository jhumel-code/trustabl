// genscoretable rewrites the generated region in docs/scoring.md with
// score tables derived from the embedded policy YAMLs.
//
// Run from the repo root:
//
//	go run ./tools/genscoretable
//
// The generated region is delimited by sentinel comments:
//
//	<!-- score-table:begin -->
//	<!-- score-table:end -->
package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/trustabl/karenctl/internal/models"
	"github.com/trustabl/karenctl/internal/rules"
)

const (
	beginMarker = "<!-- score-table:begin -->"
	endMarker   = "<!-- score-table:end -->"
	scoringDoc  = "docs/scoring.md"
)

var categoryOrder = []string{
	"claude_sdk",
	"openshell",
	"openai_sdk",
	"mcp",
	"catalog",
}

var categoryLabel = map[string]string{
	"claude_sdk":  "Claude Agent SDK (CSDK)",
	"openshell":   "OpenShell (OSH)",
	"openai_sdk":  "OpenAI Agents SDK (OAIS)",
	"mcp":         "MCP (MCP)",
	"catalog":     "Catalog capability-class (CATL)",
}

type ruleRow struct {
	id         string
	title      string
	category   string
	severity   models.Severity
	confidence float64
	score      float64
}

func baseScore(sev models.Severity, conf float64) float64 {
	return math.Round(models.BaseScoreWeight(sev)*conf*10) / 10
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

func main() {
	policies, err := rules.Load(rules.DefaultFS())
	if err != nil {
		fmt.Fprintf(os.Stderr, "load rules: %v\n", err)
		os.Exit(1)
	}

	// Collect rows, grouped by category.
	byCategory := map[string][]ruleRow{}
	for _, pf := range policies {
		cat := string(pf.Policy.Category)
		for _, r := range pf.Rules {
			byCategory[cat] = append(byCategory[cat], ruleRow{
				id:         r.ID,
				title:      r.Title,
				category:   cat,
				severity:   r.Severity,
				confidence: r.Confidence,
				score:      baseScore(r.Severity, r.Confidence),
			})
		}
	}
	// Sort each category by rule ID.
	for cat := range byCategory {
		sort.Slice(byCategory[cat], func(i, j int) bool {
			return byCategory[cat][i].id < byCategory[cat][j].id
		})
	}

	generated := generate(byCategory)

	// Read scoring.md, replace between markers, write back.
	raw, err := os.ReadFile(scoringDoc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", scoringDoc, err)
		os.Exit(1)
	}
	updated, err := replaceMarker(string(raw), beginMarker, endMarker, generated)
	if err != nil {
		fmt.Fprintf(os.Stderr, "replace marker: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(scoringDoc, []byte(updated), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", scoringDoc, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", scoringDoc)
}

func generate(byCategory map[string][]ruleRow) string {
	var b strings.Builder

	b.WriteString("## All rules scored\n")

	for _, cat := range categoryOrder {
		rows, ok := byCategory[cat]
		if !ok {
			continue
		}
		label := categoryLabel[cat]
		fmt.Fprintf(&b, "\n### %s\n\n", label)
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

func generateDistribution(byCategory map[string][]ruleRow) string {
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

	// Collect all rows across all categories in category order.
	var all []ruleRow
	for _, cat := range categoryOrder {
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
		// Wrap long lists.
		line := strings.Join(matches, ", ")
		if len(line) <= 55 {
			fmt.Fprintf(&b, "%s   %s  %s\n", band.label, bar, line)
		} else {
			// Print bar on first line, wrap items 3 per continuation line.
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

// replaceMarker replaces the content between begin and end markers (inclusive)
// with the provided replacement, wrapped in the markers.
func replaceMarker(doc, begin, end, replacement string) (string, error) {
	bi := strings.Index(doc, begin)
	ei := strings.Index(doc, end)
	if bi == -1 {
		return "", fmt.Errorf("begin marker %q not found", begin)
	}
	if ei == -1 {
		return "", fmt.Errorf("end marker %q not found", end)
	}
	if ei <= bi {
		return "", fmt.Errorf("end marker appears before begin marker")
	}
	var buf bytes.Buffer
	buf.WriteString(doc[:bi])
	buf.WriteString(begin)
	buf.WriteString("\n")
	buf.WriteString(replacement)
	buf.WriteString(end)
	buf.WriteString(doc[ei+len(end):])
	return buf.String(), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
