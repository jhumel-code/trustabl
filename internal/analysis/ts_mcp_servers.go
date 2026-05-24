package analysis

import (
	sitter "github.com/smacker/go-tree-sitter"

	"github.com/trustabl/trustabl/internal/analysis/astutil"
	"github.com/trustabl/trustabl/internal/models"
)

// DiscoverTSMCPServers extracts MCPServerDef records from TS source. Two
// recognition paths:
//   1. createSdkMcpServer({...}) calls (SDK-instance servers)
//   2. Object literals with type: "stdio"|"sse"|"http"|"sdk" inside
//      options.mcpServers records (added in a later task)
func DiscoverTSMCPServers(files []ParsedFile, onFile func(string)) []models.MCPServerDef {
	var out []models.MCPServerDef
	for _, pf := range files {
		if onFile != nil {
			onFile(pf.RelPath)
		}
		out = append(out, discoverTSMCPServersInFile(pf)...)
	}
	return out
}

func discoverTSMCPServersInFile(pf ParsedFile) []models.MCPServerDef {
	if pf.Tree == nil {
		return nil
	}
	aliases := astutil.TSImportAliases(pf.Tree.RootNode(), pf.Source, tsClaudeSDKModule)
	if len(aliases) == 0 {
		return nil
	}
	var out []models.MCPServerDef
	astutil.Walk(pf.Tree.RootNode(), func(n *sitter.Node) bool {
		if n.Type() == "call_expression" && astutil.TSCalleeText(n, pf.Source, aliases) == "createSdkMcpServer" {
			out = append(out, models.MCPServerDef{
				Class:     "createSdkMcpServer",
				Transport: "sdk",
				SDK:       models.SDKClaudeAgentSDK,
				Language:  models.LanguageTypeScript,
				FilePath:  pf.RelPath,
				Line:      int(n.StartPoint().Row) + 1,
			})
		}
		return true
	})
	return out
}
