package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelSwitchesPanelsWithKeys(t *testing.T) {
	m := NewModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := updated.(Model)
	if got.Panel != PanelAgents {
		t.Fatalf("panel = %v", got.Panel)
	}
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if updated.(Model).Panel != PanelEditor {
		t.Fatalf("expected editor")
	}
}
