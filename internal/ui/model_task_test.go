package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelOpensTaskComposerAndStartsTask(t *testing.T) {
	m := NewModel()
	updated, _ := m.Update(key("n"))
	m = updated.(Model)
	if m.Panel != PanelTask {
		t.Fatalf("panel = %q", m.Panel)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', 'd', 'd'}})
	m = updated.(Model)
	if m.TaskDescription != "add" {
		t.Fatalf("description = %q", m.TaskDescription)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.Panel != PanelAgents || m.TaskDescription != "add" {
		t.Fatalf("model = %#v", m)
	}
}

func key(value string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)} }

func TestComposerOwnsPrintableKeys(t *testing.T) {
	m := NewModel()
	m.Panel = PanelTask
	for _, r := range "add python request tiếng Việt" {
		next, cmd := m.Update(key(string(r)))
		m = next.(Model)
		if cmd != nil || m.Panel != PanelTask {
			t.Fatalf("key %q escaped composer", r)
		}
	}
	if m.TaskDescription != "add python request tiếng Việt" {
		t.Fatal(m.TaskDescription)
	}
}

func TestComposerBackspaceUnicodeAndCancel(t *testing.T) {
	m := NewModel()
	m.Panel, m.TaskDescription = PanelTask, "sửa mã"
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = next.(Model)
	if m.TaskDescription != "sửa m" {
		t.Fatalf("got %q", m.TaskDescription)
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.Panel != PanelDashboard || cmd != nil {
		t.Fatal("escape must return to dashboard")
	}
}

func TestComposerRejectsWhitespace(t *testing.T) {
	m := NewModel()
	m.Panel, m.TaskDescription = PanelTask, "   "
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if next.(Model).Panel != PanelTask || cmd != nil {
		t.Fatal("empty task accepted")
	}
}
