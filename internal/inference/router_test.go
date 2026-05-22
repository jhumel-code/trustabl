package inference

import (
	"context"
	"errors"
	"testing"
)

func TestCallWithoutKeyIsDisabled(t *testing.T) {
	r := New("")
	_, err := r.Call(context.Background(), Request{ToolASTHash: "abc"})
	if !errors.Is(err, ErrLLMDisabled) {
		t.Fatalf("Call without key: got %v, want ErrLLMDisabled", err)
	}
}

func TestCallServesFromCache(t *testing.T) {
	// A cache hit short-circuits before the key check, so even a key-less
	// router returns a cached value rather than ErrLLMDisabled.
	r := New("")
	r.cache.put("hash-1", "cached text")

	resp, err := r.Call(context.Background(), Request{ToolASTHash: "hash-1"})
	if err != nil {
		t.Fatalf("Call on cache hit: unexpected error %v", err)
	}
	if !resp.FromCache || resp.Text != "cached text" {
		t.Fatalf("Call cache hit = %+v, want {Text:\"cached text\", FromCache:true}", resp)
	}
}

func TestCacheGetPut(t *testing.T) {
	c := newCache()
	if _, ok := c.get("missing"); ok {
		t.Fatal("get on empty cache returned ok=true")
	}
	c.put("k", "v")
	got, ok := c.get("k")
	if !ok || got != "v" {
		t.Fatalf("get after put = (%q, %v), want (\"v\", true)", got, ok)
	}
}

func TestASTHashDeterministic(t *testing.T) {
	a := ASTHash("def tool(): pass")
	b := ASTHash("def tool(): pass")
	c := ASTHash("def other(): pass")
	if a != b {
		t.Fatal("ASTHash is not deterministic for identical input")
	}
	if a == c {
		t.Fatal("ASTHash collided for different input")
	}
	if len(a) != 64 {
		t.Fatalf("ASTHash length = %d, want 64 (sha256 hex)", len(a))
	}
}
