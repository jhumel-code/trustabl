package analysis

import (
	"strings"

	"github.com/trustabl/trustabl/internal/models"
)

// ClaudeBuiltinTools returns the list of Claude built-in tool names from
// AgentDefinition(tools=[...]). Returns nil if a is not a Claude AgentDef or
// the kwarg is absent / not a literal list.
func ClaudeBuiltinTools(a *models.AgentDef) []string {
	if !isClaudeAgentDef(a) {
		return nil
	}
	return readStringList(a, "tools")
}

// ClaudeDisallowedTools mirrors ClaudeBuiltinTools for the disallowedTools kwarg.
func ClaudeDisallowedTools(a *models.AgentDef) []string {
	if !isClaudeAgentDef(a) {
		return nil
	}
	return readStringList(a, "disallowedTools")
}

// ClaudePermissionMode returns the permissionMode kwarg value, or "" if absent.
func ClaudePermissionMode(a *models.AgentDef) string {
	if !isClaudeAgentDef(a) {
		return ""
	}
	kw := readChild(a, "permissionMode")
	if kw == nil || kw.Value == nil || kw.Value.Kind != models.ExprLiteralString {
		return ""
	}
	return strings.Trim(kw.Value.Text, `"'`)
}

// ClaudeMCPServers returns the mcpServers kwarg as a list of string references.
// Claude's mcpServers is documented as list[str | dict] — only the string form
// is returned here; dict entries are ignored at v1 (their shape is config, not
// references, and is not what most detectors will reason about).
func ClaudeMCPServers(a *models.AgentDef) []string {
	if !isClaudeAgentDef(a) {
		return nil
	}
	return readStringList(a, "mcpServers")
}

func isClaudeAgentDef(a *models.AgentDef) bool {
	return a != nil && a.SDK == models.SDKClaudeAgentSDK && a.Class == "AgentDefinition"
}

func readChild(a *models.AgentDef, key string) *models.KwargTree {
	if a == nil || a.Kwargs == nil {
		return nil
	}
	return a.Kwargs.Children[key]
}

func readStringList(a *models.AgentDef, key string) []string {
	kw := readChild(a, key)
	if kw == nil || kw.Value == nil || kw.Value.Kind != models.ExprList {
		return nil
	}
	out := make([]string, 0, len(kw.Value.List))
	for _, item := range kw.Value.List {
		if item.Kind != models.ExprLiteralString {
			continue
		}
		out = append(out, strings.Trim(item.Text, `"'`))
	}
	return out
}
