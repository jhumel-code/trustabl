package analysis

import (
	"sort"

	"github.com/trustabl/trustabl/internal/models"
)

// MCPServerClasses is the closed set of MCP server classes recognized by
// discovery. Source of truth: openai-agents-python/src/agents/mcp/server.py.
var MCPServerClasses = map[string]string{
	"MCPServerStdio":          "stdio",
	"MCPServerSse":            "sse",
	"MCPServerStreamableHttp": "streamable_http",
}

// IsMCPServerClass reports whether className is a recognized MCP server class.
func IsMCPServerClass(className string) bool {
	_, ok := MCPServerClasses[className]
	return ok
}

// MCPTransportFromClass returns "stdio" / "sse" / "streamable_http", or "" if
// the class is not recognized.
func MCPTransportFromClass(className string) string {
	return MCPServerClasses[className]
}

// classifyMCPServerCall inspects an ExprCall item from an mcp_servers=[...]
// list and returns an MCPServerDef + true if the callee names a known class.
// Mirrors hosted_tools.classifyHostedToolCall.
func classifyMCPServerCall(callItem models.Expr, filePath string, line int) (models.MCPServerDef, bool) {
	if callItem.Kind != models.ExprCall {
		return models.MCPServerDef{}, false
	}
	name := calleeName(callItem.Text)
	if !IsMCPServerClass(name) {
		return models.MCPServerDef{}, false
	}
	// Kwargs intentionally not captured at v1 — Expr.Text preserves the raw
	// call site for any future detector that needs the args (e.g. inspecting
	// MCPServerStdio params.command). Reparsing the kwargs from the ExprCall
	// text into MCPServerDef.Kwargs is a fast-follow if a rule needs them.
	return models.MCPServerDef{
		Class:     name,
		Transport: MCPTransportFromClass(name),
		SDK:       models.SDKOpenAIAgents,
		FilePath:  filePath,
		Line:      line,
	}, true
}

func sortMCPServers(ms []models.MCPServerDef) {
	sort.Slice(ms, func(i, j int) bool {
		if ms[i].FilePath != ms[j].FilePath {
			return ms[i].FilePath < ms[j].FilePath
		}
		if ms[i].Line != ms[j].Line {
			return ms[i].Line < ms[j].Line
		}
		return ms[i].Class < ms[j].Class
	})
}
