package astutil_test

import (
	"context"
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/trustabl/trustabl/internal/analysis/astutil"
	"github.com/trustabl/trustabl/internal/models"
)

func TestNewTSParser_ParsesHelloWorld(t *testing.T) {
	p := astutil.NewTSParser()
	if p == nil {
		t.Fatal("NewTSParser returned nil")
	}
	tree, err := p.ParseCtx(context.Background(), nil, []byte(`const x: number = 1;`))
	if err != nil || tree.RootNode().HasError() {
		t.Fatalf("parse failed: err=%v hasError=%v", err, tree.RootNode().HasError())
	}
}

func TestNewTSXParser_ParsesJSX(t *testing.T) {
	p := astutil.NewTSXParser()
	if p == nil {
		t.Fatal("NewTSXParser returned nil")
	}
	tree, err := p.ParseCtx(context.Background(), nil, []byte(`const el = <div>{x}</div>;`))
	if err != nil || tree.RootNode().HasError() {
		t.Fatalf("parse failed: err=%v hasError=%v", err, tree.RootNode().HasError())
	}
}

func TestParserForExtension(t *testing.T) {
	cases := []struct {
		path string
		want string // "typescript" | "tsx" | ""
	}{
		{"src/agent.ts", "typescript"},
		{"src/agent.mts", "typescript"},
		{"src/agent.cts", "typescript"},
		{"src/agent.tsx", "tsx"},
		{"src/agent.py", ""},
	}
	for _, c := range cases {
		got := astutil.ParserKindForExtension(c.path)
		if !strings.EqualFold(got, c.want) {
			t.Errorf("ParserKindForExtension(%q): got %q want %q", c.path, got, c.want)
		}
	}
}

func TestTSImportAliases(t *testing.T) {
	src := []byte(`
import { tool, query as q } from "@anthropic-ai/claude-agent-sdk";
import { createSdkMcpServer as mcp } from "@anthropic-ai/claude-agent-sdk";
import * as sdk from "@anthropic-ai/claude-agent-sdk";
import defaultExport from "@anthropic-ai/claude-agent-sdk";
import { z } from "zod";
`)
	p := astutil.NewTSParser()
	tree, _ := p.ParseCtx(context.Background(), nil, src)
	got := astutil.TSImportAliases(tree.RootNode(), src, "@anthropic-ai/claude-agent-sdk")
	want := map[string]string{
		"tool":          "tool",               // named, no rename
		"q":             "query",              // renamed
		"mcp":           "createSdkMcpServer", // renamed
		"sdk":           "*",                  // namespace import — sentinel "*"
		"defaultExport": "default",            // default import — sentinel "default"
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("alias[%q] = %q, want %q", k, got[k], v)
		}
	}
	if got["z"] != "" {
		t.Errorf("alias[z] should be empty (wrong module), got %q", got["z"])
	}
}

func TestTSObjectKwargs_FlatLiterals(t *testing.T) {
	src := []byte(`const x = { name: "alice", count: 3, ok: true, missing: null };`)
	p := astutil.NewTSParser()
	tree, _ := p.ParseCtx(context.Background(), nil, src)
	obj := findFirstObjectLiteral(tree.RootNode())
	if obj == nil {
		t.Fatal("no object literal found")
	}
	kt := astutil.TSObjectKwargs(obj, src)
	checkLeaf := func(k string, wantKind models.ExprKind, wantText string) {
		t.Helper()
		c, ok := kt.Children[k]
		if !ok {
			t.Errorf("missing key %q", k)
			return
		}
		if c.Value == nil || c.Value.Kind != wantKind || c.Value.Text != wantText {
			t.Errorf("key %q: got %+v, want kind=%s text=%q", k, c.Value, wantKind, wantText)
		}
	}
	checkLeaf("name", models.ExprLiteralString, `"alice"`)
	checkLeaf("count", models.ExprLiteralInt, "3")
	checkLeaf("ok", models.ExprLiteralBool, "true")
	checkLeaf("missing", models.ExprLiteralNone, "null")
}

func TestTSObjectKwargs_NestedAndLists(t *testing.T) {
	src := []byte(`const x = { tools: ["Read", "Bash"], cfg: { strict: true } };`)
	p := astutil.NewTSParser()
	tree, _ := p.ParseCtx(context.Background(), nil, src)
	obj := findFirstObjectLiteral(tree.RootNode())
	kt := astutil.TSObjectKwargs(obj, src)
	tools := kt.Children["tools"]
	if tools == nil || tools.Value == nil || tools.Value.Kind != models.ExprList {
		t.Fatalf("tools: got %+v, want list", tools)
	}
	if len(tools.Value.List) != 2 || tools.Value.List[0].Text != `"Read"` {
		t.Errorf("tools list: %+v", tools.Value.List)
	}
	cfg := kt.Children["cfg"]
	if cfg == nil || cfg.Children["strict"] == nil {
		t.Fatalf("cfg.strict: got %+v", cfg)
	}
}

// findFirstObjectLiteral is a tiny test helper. Walks until the first
// "object" node (the TS grammar uses "object" for object literals).
func findFirstObjectLiteral(n *sitter.Node) *sitter.Node {
	var out *sitter.Node
	astutil.Walk(n, func(x *sitter.Node) bool {
		if out != nil {
			return false
		}
		if x.Type() == "object" {
			out = x
			return false
		}
		return true
	})
	return out
}
