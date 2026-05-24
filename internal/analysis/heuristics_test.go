package analysis

import (
	"testing"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/trustabl/trustabl/internal/analysis/astutil"
)

func firstFunctionNode(t *testing.T, src string) (*sitter.Node, []byte) {
	t.Helper()
	b := []byte(src)
	tree, err := astutil.Parse(b)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var fn *sitter.Node
	astutil.Walk(tree.RootNode(), func(n *sitter.Node) bool {
		if fn != nil {
			return false
		}
		if n.Type() == "function_definition" {
			fn = n
			return false
		}
		return true
	})
	if fn == nil {
		t.Fatal("no function_definition found")
	}
	return fn, b
}

func TestResolveClientAliases(t *testing.T) {
	src := `
def tool():
    s = requests.Session()
    c = httpx.Client()
    sess = aiohttp.ClientSession()
    x = compute()
    with httpx.AsyncClient() as ac:
        ac.get("u")
    s.get("u")
`
	fn, b := firstFunctionNode(t, src)
	got := ResolveClientAliases(fn, b)
	want := map[string]string{
		"s":    "requests",
		"c":    "httpx",
		"sess": "aiohttp",
		"ac":   "httpx",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d aliases %v, want %d %v", len(got), got, len(want), want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("alias %q = %q, want %q", k, got[k], v)
		}
	}
	if _, ok := got["x"]; ok {
		t.Errorf("non-client assignment x should not be an alias")
	}
}

func TestResolveClientAliases_LastWriteWins(t *testing.T) {
	src := `
def tool():
    s = requests.Session()
    s = compute()
    s.get("u")
`
	fn, b := firstFunctionNode(t, src)
	got := ResolveClientAliases(fn, b)
	if _, ok := got["s"]; ok {
		t.Errorf("s rebound to non-client should not remain an alias; got %v", got)
	}
}
