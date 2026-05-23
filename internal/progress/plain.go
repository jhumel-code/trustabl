package progress

import (
	"fmt"
	"io"
)

// plainReporter writes one static "[key] summary" line per finished phase. No
// ANSI, no animation — safe for piped output and CI logs.
type plainReporter struct {
	w   io.Writer
	key string
}

// NewPlain returns a Reporter that writes static phase-summary lines to w.
func NewPlain(w io.Writer) Reporter { return &plainReporter{w: w} }

func (r *plainReporter) StartPhase(key, _ string) { r.key = key }
func (r *plainReporter) SetTotal(int)             {}
func (r *plainReporter) Advance(string)           {}
func (r *plainReporter) EndPhase(summary string)  { fmt.Fprintf(r.w, "[%s] %s\n", r.key, summary) }
func (r *plainReporter) Fatal(err error)          { fmt.Fprintf(r.w, "[%s] failed: %v\n", r.key, err) }
