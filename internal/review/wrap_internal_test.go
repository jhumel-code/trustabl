package review

import (
	"strings"
	"testing"
)

func TestWrapAt(t *testing.T) {
	// Short text shorter than the limit is returned on one line.
	if got := wrapAt("a short line", 40); got != "a short line" {
		t.Fatalf("wrapAt short = %q, want unchanged", got)
	}

	// Long text wraps: at least one newline appears, and no single rendered
	// line (ignoring the indent prefix) exceeds the limit by a whole word.
	long := strings.Repeat("word ", 40)
	got := wrapAt(long, 20)
	if !strings.Contains(got, "\n") {
		t.Fatalf("wrapAt(long, 20) did not wrap: %q", got)
	}

	// Empty input yields empty output, not a panic.
	if got := wrapAt("", 10); got != "" {
		t.Fatalf("wrapAt(\"\") = %q, want empty", got)
	}
}
