package analysis

import "github.com/trustabl/trustabl/internal/models"

// ADKHostedToolClasses is the closed set of Google ADK built-in tool classes
// recognized by discovery. Source of truth: google/adk-python's
// src/google/adk/tools/ directory.
var ADKHostedToolClasses = map[string]bool{
	"BashTool":                  true,
	"GoogleSearchTool":          true,
	"VertexAiSearchTool":        true,
	"LangchainTool":             true,
	"CrewaiTool":                true,
	"AgentTool":                 true,
	"LongRunningTool":           true,
	"LoadWebPage":               true,
	"ExitLoopTool":              true,
	"GoogleMapsGroundingTool":   true,
	"UrlContextTool":            true,
	"DiscoveryEngineSearchTool": true,
	"EnterpriseSearchTool":      true,
}

// IsADKHostedToolClass reports whether className is a recognized ADK
// built-in tool class.
func IsADKHostedToolClass(className string) bool { return ADKHostedToolClasses[className] }

// classifyADKHostedToolCall inspects an ExprCall item from an ADK agent's
// tools=[...] list and returns a HostedToolDef + true if the callee names an
// ADK built-in tool class.
func classifyADKHostedToolCall(callItem models.Expr, filePath string, line int) (models.HostedToolDef, bool) {
	if callItem.Kind != models.ExprCall {
		return models.HostedToolDef{}, false
	}
	name := calleeName(callItem.Text)
	if !IsADKHostedToolClass(name) {
		return models.HostedToolDef{}, false
	}
	return models.HostedToolDef{
		Class:    name,
		SDK:      models.SDKGoogleADK,
		FilePath: filePath,
		Line:     line,
	}, true
}
