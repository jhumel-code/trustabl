package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/trustabl/trustabl/internal/models"
)

// permRuleRegex matches "Tool" or "Tool(pattern)". Tool is alphanumeric and
// starts uppercase. Pattern is everything up to the final ')'.
var permRuleRegex = regexp.MustCompile(`^([A-Z][A-Za-z]+)(?:\(([^)]*)\))?$`)

// mcpToolLiteral matches the form mcp__<server>__<tool> (no parens).
var mcpToolLiteral = regexp.MustCompile(`^mcp__(.+)$`)

// ParsePermissionRule parses one entry from the allow/deny/ask lists into a
// typed PermissionRule. Unknown shapes still emit a PermissionRule with the
// Raw field set so detectors can surface them; Tool will be empty.
func ParsePermissionRule(raw string) models.PermissionRule {
	rule := models.PermissionRule{Raw: raw}
	if m := mcpToolLiteral.FindStringSubmatch(raw); m != nil {
		rule.Tool = "MCP"
		rule.Pattern = m[1]
		return rule
	}
	if m := permRuleRegex.FindStringSubmatch(raw); m != nil {
		rule.Tool = m[1]
		rule.Pattern = m[2]
	}
	return rule
}

// DiscoverClaudeSettings reads every ComponentClaudeSettings file and parses
// the permissions block. Files that fail JSON parse are skipped silently.
func DiscoverClaudeSettings(manifest models.ScanManifest) []models.ClaudeSettings {
	var out []models.ClaudeSettings
	for _, c := range manifest.Components {
		if c.Kind != models.ComponentClaudeSettings {
			continue
		}
		full := filepath.Join(manifest.RepoRoot, c.Path)
		raw, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		var parsed claudeSettingsFile
		if err := json.Unmarshal(raw, &parsed); err != nil {
			continue
		}
		out = append(out, models.ClaudeSettings{
			FilePath:        filepath.ToSlash(c.Path),
			Permissions:     parsePermissionsBlock(parsed.Permissions),
			DefaultMode:     parsed.Permissions.DefaultMode,
			AdditionalDirs:  parsed.Permissions.AdditionalDirectories,
			HasEnvBlock:     parsed.Env != nil,
			HasHooks:        parsed.Hooks != nil,
			HasSandboxBlock: parsed.Sandbox != nil,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FilePath < out[j].FilePath })
	return out
}

type claudeSettingsFile struct {
	Permissions permissionsRaw  `json:"permissions"`
	Env         json.RawMessage `json:"env"`
	Hooks       json.RawMessage `json:"hooks"`
	Sandbox     json.RawMessage `json:"sandbox"`
}

type permissionsRaw struct {
	Allow                 []string `json:"allow"`
	Deny                  []string `json:"deny"`
	Ask                   []string `json:"ask"`
	DefaultMode           string   `json:"defaultMode"`
	AdditionalDirectories []string `json:"additionalDirectories"`
}

func parsePermissionsBlock(p permissionsRaw) models.ClaudePermissions {
	return models.ClaudePermissions{
		Allow: parseRules(p.Allow),
		Deny:  parseRules(p.Deny),
		Ask:   parseRules(p.Ask),
	}
}

func parseRules(raw []string) []models.PermissionRule {
	if len(raw) == 0 {
		return nil
	}
	out := make([]models.PermissionRule, 0, len(raw))
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		out = append(out, ParsePermissionRule(r))
	}
	return out
}
