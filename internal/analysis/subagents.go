package analysis

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/trustabl/trustabl/internal/models"
)

// DiscoverSubagents reads every ComponentSubagent in the manifest, parses the
// YAML frontmatter between the leading `---` markers, and returns one
// SubagentDef per file with frontmatter. Files without frontmatter are
// skipped silently (a subagent without frontmatter is just markdown
// documentation — it has no name/tools to act on).
func DiscoverSubagents(manifest models.ScanManifest) []models.SubagentDef {
	var out []models.SubagentDef
	for _, c := range manifest.Components {
		if c.Kind != models.ComponentSubagent {
			continue
		}
		full := filepath.Join(manifest.RepoRoot, c.Path)
		raw, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		fm, ok := extractFrontmatter(raw)
		if !ok {
			continue
		}
		var parsed subagentFrontmatter
		if err := yaml.Unmarshal(fm, &parsed); err != nil {
			continue
		}
		out = append(out, models.SubagentDef{
			Name:        parsed.Name,
			Description: parsed.Description,
			Model:       parsed.Model,
			FilePath:    c.Path,
			Tools:       splitToolsField(parsed.Tools),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FilePath < out[j].FilePath })
	return out
}

type subagentFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Tools       string `yaml:"tools"` // comma-separated string in the wild
	Model       string `yaml:"model"`
}

// extractFrontmatter pulls the YAML block between leading "---\n" and the
// next "---" line. Returns (block, true) on success, (nil, false) if the
// file does not start with "---".
func extractFrontmatter(raw []byte) ([]byte, bool) {
	hasLF := bytes.HasPrefix(raw, []byte("---\n"))
	hasCRLF := bytes.HasPrefix(raw, []byte("---\r\n"))
	if !hasLF && !hasCRLF {
		return nil, false
	}
	rest := raw[4:]
	if hasCRLF {
		rest = raw[5:]
	}
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		return nil, false
	}
	return rest[:end], true
}

func splitToolsField(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
