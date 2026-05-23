package progress

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelEndPhasePrintsAndClears(t *testing.T) {
	m := newModel()
	m2, _ := m.Update(startPhaseMsg{key: "recon", label: "Recon"})
	m3, cmd := m2.(model).Update(endPhaseMsg{summary: "18 files"})
	mm := m3.(model)
	if mm.active {
		t.Error("phase still active after endPhaseMsg")
	}
	if cmd == nil {
		t.Fatal("endPhaseMsg should return a print command")
	}
}

func TestModelDoneQuits(t *testing.T) {
	m := newModel()
	_, cmd := m.Update(doneMsg{})
	if cmd == nil {
		t.Fatal("doneMsg should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("doneMsg command did not produce tea.QuitMsg")
	}
}

func TestModelAdvanceTracksCount(t *testing.T) {
	m := newModel()
	m2, _ := m.Update(startPhaseMsg{key: "inventory", label: "Inventory"})
	m3, _ := m2.(model).Update(setTotalMsg{n: 3})
	m4, _ := m3.(model).Update(advanceMsg{detail: "a.py"})
	mm := m4.(model)
	if mm.count != 1 || mm.total != 3 || mm.detail != "a.py" {
		t.Errorf("after advance: count=%d total=%d detail=%q", mm.count, mm.total, mm.detail)
	}
}
