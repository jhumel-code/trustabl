package rules

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/trustabl/karenctl/internal/analysis"
	"github.com/trustabl/karenctl/internal/analysis/astutil"
	"github.com/trustabl/karenctl/internal/models"
)

// ─── bool predicates ─────────────────────────────────────────────────────────

func PredHasDocstring(t models.ToolDef) bool {
	return strings.TrimSpace(t.Description) != ""
}

func PredHasParams(t models.ToolDef) bool {
	return len(t.ParamNames) > 0
}

func PredHasTypedParams(t models.ToolDef) bool {
	return t.HasInputSchema
}

func PredHasRaise(t models.ToolDef, pf analysis.ParsedFile) bool {
	root := findFunctionNode(t, pf)
	if root == nil {
		return false
	}
	return len(astutil.FindAll(root, "raise_statement")) > 0
}

func PredHasTryExcept(t models.ToolDef, pf analysis.ParsedFile) bool {
	root := findFunctionNode(t, pf)
	if root == nil {
		return false
	}
	return len(astutil.FindAll(root, "try_statement")) > 0
}

func PredHasShellCall(t models.ToolDef, pf analysis.ParsedFile) bool {
	root := findFunctionNode(t, pf)
	if root == nil {
		return false
	}
	found := false
	astutil.Walk(root, func(n *sitter.Node) bool {
		if found {
			return false
		}
		if n.Type() != "call" {
			return true
		}
		fn := n.ChildByFieldName("function")
		if fn == nil {
			return true
		}
		c := astutil.NodeText(fn, pf.Source)
		if strings.HasPrefix(c, "subprocess.") || c == "os.system" || c == "os.popen" {
			found = true
			return false
		}
		return true
	})
	return found
}

func PredHasWriteCall(t models.ToolDef, pf analysis.ParsedFile) bool {
	root := findFunctionNode(t, pf)
	if root == nil {
		return false
	}
	found := false
	astutil.Walk(root, func(n *sitter.Node) bool {
		if found {
			return false
		}
		if n.Type() != "call" {
			return true
		}
		fn := n.ChildByFieldName("function")
		if fn == nil {
			return true
		}
		callee := astutil.NodeText(fn, pf.Source)
		if callee == "open" {
			args := n.ChildByFieldName("arguments")
			if args != nil {
				text := astutil.NodeText(args, pf.Source)
				if strings.Contains(text, `"w"`) || strings.Contains(text, `'w'`) ||
					strings.Contains(text, `"a"`) || strings.Contains(text, `'a'`) ||
					strings.Contains(text, `"x"`) || strings.Contains(text, `'x'`) {
					found = true
					return false
				}
			}
			return true
		}
		if callee == "shutil.copy" || callee == "shutil.copy2" ||
			callee == "shutil.move" || callee == "shutil.rmtree" {
			found = true
			return false
		}
		return true
	})
	return found
}

func PredHasDynamicURLCall(t models.ToolDef, pf analysis.ParsedFile) bool {
	root := findFunctionNode(t, pf)
	if root == nil {
		return false
	}
	found := false
	astutil.Walk(root, func(n *sitter.Node) bool {
		if found {
			return false
		}
		if n.Type() != "call" {
			return true
		}
		fn := n.ChildByFieldName("function")
		if fn == nil {
			return true
		}
		if !isHTTPCall(astutil.NodeText(fn, pf.Source)) {
			return true
		}
		args := n.ChildByFieldName("arguments")
		if args == nil {
			return true
		}
		if int(args.NamedChildCount()) > 0 {
			first := args.NamedChild(0)
			if first.Type() != "string" {
				found = true
			} else {
				for i := 0; i < int(first.NamedChildCount()); i++ {
					if first.NamedChild(i).Type() == "interpolation" {
						found = true
						break
					}
				}
			}
		}
		return !found
	})
	return found
}

// ─── string-list predicates ───────────────────────────────────────────────────

func PredNameIn(names []string, t models.ToolDef) bool {
	lower := strings.ToLower(t.Name)
	for _, n := range names {
		if lower == strings.ToLower(n) {
			return true
		}
	}
	return false
}

func PredNameHasPrefix(prefixes []string, t models.ToolDef) bool {
	lower := strings.ToLower(t.Name)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func PredHasBodyText(needles []string, t models.ToolDef, pf analysis.ParsedFile) bool {
	root := findFunctionNode(t, pf)
	if root == nil {
		return false
	}
	body := astutil.NodeText(root, pf.Source)
	for _, needle := range needles {
		if strings.Contains(body, needle) {
			return true
		}
	}
	return false
}

func PredParamNameMatches(expr ParamNameMatchExpr, t models.ToolDef) bool {
	for _, p := range t.ParamNames {
		lower := strings.ToLower(p)
		for _, e := range expr.Exact {
			if lower == strings.ToLower(e) {
				return true
			}
		}
		for _, c := range expr.Contains {
			if strings.Contains(lower, strings.ToLower(c)) {
				return true
			}
		}
		for _, s := range expr.Suffixes {
			if strings.HasSuffix(lower, strings.ToLower(s)) {
				return true
			}
		}
		for _, pr := range expr.Prefixes {
			if strings.HasPrefix(lower, strings.ToLower(pr)) {
				return true
			}
		}
	}
	return false
}

// ─── call-site predicates ─────────────────────────────────────────────────────

func PredCallWithoutKwarg(expr CallWithoutKwargExpr, t models.ToolDef, pf analysis.ParsedFile) bool {
	root := findFunctionNode(t, pf)
	if root == nil {
		return false
	}
	calleeSet := make(map[string]struct{}, len(expr.Callees))
	for _, c := range expr.Callees {
		calleeSet[c] = struct{}{}
	}
	found := false
	astutil.Walk(root, func(n *sitter.Node) bool {
		if found {
			return false
		}
		if n.Type() != "call" {
			return true
		}
		fn := n.ChildByFieldName("function")
		if fn == nil {
			return true
		}
		if _, ok := calleeSet[astutil.NodeText(fn, pf.Source)]; !ok {
			return true
		}
		if !hasKwarg(n, pf.Source, expr.Missing) {
			found = true
		}
		return !found
	})
	return found
}

func PredCallWithKwargValue(expr CallWithKwargValueExpr, t models.ToolDef, pf analysis.ParsedFile) bool {
	root := findFunctionNode(t, pf)
	if root == nil {
		return false
	}
	calleeSet := make(map[string]struct{}, len(expr.Callees))
	for _, c := range expr.Callees {
		calleeSet[c] = struct{}{}
	}
	found := false
	astutil.Walk(root, func(n *sitter.Node) bool {
		if found {
			return false
		}
		if n.Type() != "call" {
			return true
		}
		fn := n.ChildByFieldName("function")
		if fn == nil {
			return true
		}
		callee := astutil.NodeText(fn, pf.Source)
		matches := false
		if _, ok := calleeSet[callee]; ok {
			matches = true
		}
		if !matches && expr.CalleePrefix != "" && strings.HasPrefix(callee, expr.CalleePrefix) {
			matches = true
		}
		if !matches {
			return true
		}
		args := n.ChildByFieldName("arguments")
		if args == nil {
			return true
		}
		astutil.Walk(args, func(kn *sitter.Node) bool {
			if kn.Type() != "keyword_argument" {
				return true
			}
			kname := kn.ChildByFieldName("name")
			kval := kn.ChildByFieldName("value")
			if kname == nil || kval == nil {
				return true
			}
			if astutil.NodeText(kname, pf.Source) == expr.Kwarg &&
				astutil.NodeText(kval, pf.Source) == expr.Value {
				found = true
				return false
			}
			return true
		})
		return !found
	})
	return found
}

func PredCallUsesParam(expr CallUsesParamExpr, t models.ToolDef, pf analysis.ParsedFile) bool {
	root := findFunctionNode(t, pf)
	if root == nil {
		return false
	}
	pathish := make(map[string]struct{})
	for _, p := range t.ParamNames {
		if isPathishParam(p) {
			pathish[p] = struct{}{}
		}
	}
	if len(pathish) == 0 {
		return false
	}
	calleeSet := make(map[string]struct{}, len(expr.Callees))
	for _, c := range expr.Callees {
		calleeSet[c] = struct{}{}
	}
	found := false
	astutil.Walk(root, func(n *sitter.Node) bool {
		if found {
			return false
		}
		if n.Type() != "call" {
			return true
		}
		fn := n.ChildByFieldName("function")
		if fn == nil {
			return true
		}
		callee := astutil.NodeText(fn, pf.Source)
		matches := false
		if _, ok := calleeSet[callee]; ok {
			matches = true
		}
		if !matches {
			for _, pref := range expr.CalleePrefixes {
				if strings.HasPrefix(callee, pref) {
					matches = true
					break
				}
			}
		}
		if !matches {
			return true
		}
		args := n.ChildByFieldName("arguments")
		if args == nil {
			return true
		}
		astutil.Walk(args, func(arg *sitter.Node) bool {
			if arg.Type() == "identifier" {
				if _, ok := pathish[astutil.NodeText(arg, pf.Source)]; ok {
					found = true
					return false
				}
			}
			return true
		})
		return !found
	})
	return found
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func findFunctionNode(t models.ToolDef, pf analysis.ParsedFile) *sitter.Node {
	if pf.Tree == nil {
		return nil
	}
	var match *sitter.Node
	astutil.Walk(pf.Tree.RootNode(), func(n *sitter.Node) bool {
		if match != nil {
			return false
		}
		if n.Type() != "function_definition" {
			return true
		}
		if astutil.NodeLine(n) == t.Line && astutil.FunctionName(n, pf.Source) == t.Name {
			match = n
			return false
		}
		return true
	})
	return match
}

func hasKwarg(call *sitter.Node, src []byte, name string) bool {
	args := call.ChildByFieldName("arguments")
	if args == nil {
		return false
	}
	found := false
	astutil.Walk(args, func(n *sitter.Node) bool {
		if n.Type() != "keyword_argument" {
			return true
		}
		k := n.ChildByFieldName("name")
		if k != nil && astutil.NodeText(k, src) == name {
			found = true
			return false
		}
		return true
	})
	return found
}

func isHTTPCall(callee string) bool {
	switch callee {
	case "requests.get", "requests.post", "requests.put", "requests.delete",
		"requests.patch", "requests.head", "requests.request",
		"requests.Session.get", "requests.Session.post",
		"httpx.get", "httpx.post", "httpx.put", "httpx.delete",
		"httpx.patch", "httpx.head", "httpx.request",
		"httpx.AsyncClient", "httpx.Client",
		"urllib.request.urlopen", "aiohttp.ClientSession.get",
		"aiohttp.ClientSession.post":
		return true
	}
	return false
}

func isPathishParam(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "path", "file", "filename", "filepath", "dir", "directory":
		return true
	}
	return strings.HasSuffix(lower, "_path") ||
		strings.HasSuffix(lower, "_file") ||
		strings.HasSuffix(lower, "_dir") ||
		strings.HasSuffix(lower, "_directory") ||
		strings.HasPrefix(lower, "file_") ||
		strings.HasPrefix(lower, "path_")
}
