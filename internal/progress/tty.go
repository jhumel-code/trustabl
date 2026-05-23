package progress

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Messages mirror the Reporter methods; the TTYReporter sends them via p.Send.
type startPhaseMsg struct{ key, label string }
type setTotalMsg struct{ n int }
type advanceMsg struct{ detail string }
type endPhaseMsg struct{ summary string }
type doneMsg struct{}
type fatalMsg struct{ err error }

// model is the bubbletea model rendering the active phase. Completed phases are
// printed as persistent lines via tea.Println (above the live view).
type model struct {
	spinner spinner.Model
	bar     progress.Model

	active bool
	key    string
	label  string
	total  int
	count  int
	detail string
	err    error
}

func newModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return model{
		spinner: s,
		bar:     progress.New(progress.WithDefaultGradient(), progress.WithWidth(24)),
	}
}

func (m model) Init() tea.Cmd { return m.spinner.Tick }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startPhaseMsg:
		m.active, m.key, m.label = true, msg.key, msg.label
		m.total, m.count, m.detail = 0, 0, ""
		return m, nil
	case setTotalMsg:
		m.total = msg.n
		return m, nil
	case advanceMsg:
		m.count++
		m.detail = msg.detail
		return m, nil
	case endPhaseMsg:
		m.active = false
		line := fmt.Sprintf("[%s] %s", m.key, msg.summary)
		return m, tea.Println(line)
	case fatalMsg:
		m.err = msg.err
		return m, tea.Sequence(tea.Println(fmt.Sprintf("[%s] failed: %v", m.key, msg.err)), tea.Quit)
	case doneMsg:
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) View() string {
	if !m.active {
		return ""
	}
	if m.total > 0 {
		pct := float64(m.count) / float64(m.total)
		return fmt.Sprintf("%s %s %s %d/%d  %s\n",
			m.spinner.View(), m.label, m.bar.ViewAs(pct), m.count, m.total, m.detail)
	}
	return fmt.Sprintf("%s %s\n", m.spinner.View(), m.label)
}

// TTYReporter forwards Reporter calls to a running bubbletea program. It
// implements Reporter and adds Run/Done to drive the render loop.
type TTYReporter struct{ p *tea.Program }

// NewTTY builds a TTYReporter rendering to w (stderr). The caller runs the loop
// with Run() on the main goroutine while emitting events from another goroutine.
func NewTTY(w io.Writer) *TTYReporter {
	p := tea.NewProgram(newModel(), tea.WithOutput(w))
	return &TTYReporter{p: p}
}

// Run renders until Done/Fatal triggers quit. Call on the main goroutine.
func (r *TTYReporter) Run() error { _, err := r.p.Run(); return err }

// Done signals the render loop to stop (call after the job finishes).
func (r *TTYReporter) Done() { r.p.Send(doneMsg{}) }

func (r *TTYReporter) StartPhase(key, label string) { r.p.Send(startPhaseMsg{key, label}) }
func (r *TTYReporter) SetTotal(n int)               { r.p.Send(setTotalMsg{n}) }
func (r *TTYReporter) Advance(detail string)        { r.p.Send(advanceMsg{detail}) }
func (r *TTYReporter) EndPhase(summary string)      { r.p.Send(endPhaseMsg{summary}) }
func (r *TTYReporter) Fatal(err error)              { r.p.Send(fatalMsg{err}) }

var _ Reporter = (*TTYReporter)(nil)
