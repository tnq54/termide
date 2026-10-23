package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelOpensTaskComposerAndStartsTask(t *testing.T) {
	m := NewModel()
	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	if m.Panel != PanelTask { t.Fatalf("panel = %q", m.Panel) }
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', 'd', 'd'}})
	m = updated.(Model)
	if m.TaskDescription != "add" { t.Fatalf("description = %q", m.TaskDescription) }
	updated, _ = m.Update(key("enter"))
	m = updated.(Model)
	if m.Panel != PanelAgents || m.TaskDescription != "add" { t.Fatalf("model = %#v", m) }
}

func key(value string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)} }
