package analysis

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/trustabl/trustabl/internal/analysis/astutil"
	"github.com/trustabl/trustabl/internal/models"
)

// DiscoverTSAgents extracts AgentDef records from TS source. Two shapes:
//   1. Inline inside query({ options: { agents: { ... } } })
//   2. Typed-const declarations: const x: AgentDefinition = {...}
// (Typed-const shape added in a later task.)
func DiscoverTSAgents(files []ParsedFile, onFile func(string)) []models.AgentDef {
	var out []models.AgentDef
	for _, pf := range files {
		if onFile != nil {
			onFile(pf.RelPath)
		}
		out = append(out, discoverTSAgentsInFile(pf)...)
	}
	return out
}

func discoverTSAgentsInFile(pf ParsedFile) []models.AgentDef {
	if pf.Tree == nil {
		return nil
	}
	aliases := astutil.TSImportAliases(pf.Tree.RootNode(), pf.Source, tsClaudeSDKModule)
	if len(aliases) == 0 {
		return nil // import gate
	}
	var out []models.AgentDef
	astutil.Walk(pf.Tree.RootNode(), func(n *sitter.Node) bool {
		switch n.Type() {
		case "call_expression":
			if astutil.TSCalleeText(n, pf.Source, aliases) == "query" {
				out = append(out, extractInlineAgentsFromQuery(n, pf)...)
			}
		case "variable_declarator":
			if a, ok := extractTypedConstAgent(n, pf); ok {
				out = append(out, a)
			}
		}
		return true
	})
	return out
}

func extractInlineAgentsFromQuery(call *sitter.Node, pf ParsedFile) []models.AgentDef {
	args := call.ChildByFieldName("arguments")
	if args == nil || args.NamedChildCount() < 1 {
		return nil
	}
	root := args.NamedChild(0)
	if root.Type() != "object" {
		return nil
	}
	options := getObjectProperty(root, "options", pf.Source)
	if options == nil || options.Type() != "object" {
		return nil
	}
	agentsObj := getObjectProperty(options, "agents", pf.Source)
	if agentsObj == nil || agentsObj.Type() != "object" {
		return nil
	}
	var out []models.AgentDef
	for i := 0; i < int(agentsObj.NamedChildCount()); i++ {
		prop := agentsObj.NamedChild(i)
		if prop.Type() != "pair" {
			continue
		}
		keyNode := prop.ChildByFieldName("key")
		valNode := prop.ChildByFieldName("value")
		if keyNode == nil || valNode == nil {
			continue
		}
		var name string
		switch keyNode.Type() {
		case "property_identifier":
			name = astutil.NodeText(keyNode, pf.Source)
		case "string":
			raw := astutil.NodeText(keyNode, pf.Source)
			if len(raw) >= 2 {
				name = raw[1 : len(raw)-1]
			}
		}
		agent := models.AgentDef{
			SDK:      models.SDKClaudeAgentSDK,
			Class:    "AgentDefinition",
			Language: models.LanguageTypeScript,
			FilePath: pf.RelPath,
			Line:     int(prop.StartPoint().Row) + 1,
			Name:     name,
		}
		if valNode.Type() != "object" {
			agent.Opaque = true
		} else {
			agent.Kwargs = astutil.TSObjectKwargs(valNode, pf.Source)
		}
		populateTSAgentToolRefs(&agent)
		out = append(out, agent)
	}
	return out
}

// getObjectProperty returns the value node of `obj.prop` if obj is an object
// literal with a literal property_identifier or string key matching `key`;
// nil otherwise.
func getObjectProperty(obj *sitter.Node, key string, src []byte) *sitter.Node {
	if obj == nil || obj.Type() != "object" {
		return nil
	}
	for i := 0; i < int(obj.NamedChildCount()); i++ {
		prop := obj.NamedChild(i)
		if prop.Type() != "pair" {
			continue
		}
		k := prop.ChildByFieldName("key")
		v := prop.ChildByFieldName("value")
		if k == nil || v == nil {
			continue
		}
		var kname string
		switch k.Type() {
		case "property_identifier":
			kname = astutil.NodeText(k, src)
		case "string":
			raw := astutil.NodeText(k, src)
			if len(raw) >= 2 {
				kname = raw[1 : len(raw)-1]
			}
		}
		if kname == key {
			return v
		}
	}
	return nil
}

func extractTypedConstAgent(decl *sitter.Node, pf ParsedFile) (models.AgentDef, bool) {
	nameNode := decl.ChildByFieldName("name")
	typeNode := decl.ChildByFieldName("type")
	valueNode := decl.ChildByFieldName("value")
	if nameNode == nil || typeNode == nil || valueNode == nil {
		return models.AgentDef{}, false
	}
	if nameNode.Type() != "identifier" || valueNode.Type() != "object" {
		return models.AgentDef{}, false
	}
	// type field text looks like ": AgentDefinition" — substring check.
	if !strings.Contains(astutil.NodeText(typeNode, pf.Source), "AgentDefinition") {
		return models.AgentDef{}, false
	}
	name := astutil.NodeText(nameNode, pf.Source)
	agent := models.AgentDef{
		SDK:      models.SDKClaudeAgentSDK,
		Class:    "AgentDefinition",
		Language: models.LanguageTypeScript,
		FilePath: pf.RelPath,
		Line:     int(decl.StartPoint().Row) + 1,
		Name:     name,
		VarName:  name,
		Kwargs:   astutil.TSObjectKwargs(valueNode, pf.Source),
	}
	populateTSAgentToolRefs(&agent)
	return agent, true
}

// populateTSAgentToolRefs reads agent.Kwargs.Children["tools"] (if it's a
// list of string literals) and appends one ToolRef per entry. Builtin tool
// names like "Read"/"Bash" stay as strings — they're not resolved to
// ToolDefs (which represent user-defined tools).
func populateTSAgentToolRefs(a *models.AgentDef) {
	if a.Kwargs == nil {
		return
	}
	tools := a.Kwargs.Children["tools"]
	if tools == nil || tools.Value == nil || tools.Value.Kind != models.ExprList {
		return
	}
	for _, item := range tools.Value.List {
		if item.Kind != models.ExprLiteralString {
			continue
		}
		raw := item.Text
		if len(raw) < 2 {
			continue
		}
		name := raw[1 : len(raw)-1]
		a.ToolRefs = append(a.ToolRefs, models.ToolRef{Name: name})
	}
}
