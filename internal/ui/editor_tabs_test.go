package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func twoFileFixture(t *testing.T) (Model, string) {
	t.Helper()
	m, root := editorFixture(t)
	if err := os.WriteFile(filepath.Join(root, "util.py"), []byte("x = 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m = applyCommand(t, m, key("e"))
	return m, root
}

func openSelected(t *testing.T, m Model) Model {
	t.Helper()
	return applyCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
}

func TestEditorTabsPreservePerTabEdits(t *testing.T) {
	m, _ := twoFileFixture(t)
	m = openSelected(t, m)
	first := m.files.document.Path
	// Back to the file list without losing the clean tab, then open the other file.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if len(m.files.tabs) != 1 {
		t.Fatal("first tab was not stashed")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	m = openSelected(t, m)
	if m.files.document == nil || m.files.document.Path == first {
		t.Fatal("second file did not open in a new tab")
	}
	second := m.files.document.Path
	next, _ = m.Update(key("edit-one"))
	m = next.(Model)
	if !strings.Contains(m.files.input.Value(), "edit-one") {
		t.Fatal("edit did not apply")
	}
	// Switch back: first tab is untouched.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = next.(Model)
	if m.files.document == nil || m.files.document.Path != first {
		t.Fatalf("did not switch back: %#v", m.files.document)
	}
	if strings.Contains(m.files.input.Value(), "edit-one") {
		t.Fatal("second tab edit leaked into first tab")
	}
	// Cycle forward again: second tab keeps its edit.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = next.(Model)
	if m.files.document == nil || m.files.document.Path != second {
		t.Fatal("cycle did not return to second tab")
	}
	if !strings.Contains(m.files.input.Value(), "edit-one") {
		t.Fatal("tab edit lost after switch")
	}
}

func TestEditorEscStashesCleanTab(t *testing.T) {
	m, _ := twoFileFixture(t)
	m = openSelected(t, m)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.files.document != nil {
		t.Fatal("expected file list after escape")
	}
	if len(m.files.tabs) != 1 {
		t.Fatalf("clean tab was not stashed: %#v", m.files.tabs)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	m = openSelected(t, m)
	view := m.View()
	if !strings.Contains(view, "main.py") || !strings.Contains(view, "util.py") {
		t.Fatalf("tab bar missing stashed tab:\n%s", view)
	}
}

func TestEditorDiscardDropsTab(t *testing.T) {
	m, _ := twoFileFixture(t)
	m = openSelected(t, m)
	next, _ := m.Update(key("unsaved"))
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = next.(Model)
	if m.files.document != nil {
		t.Fatal("discarded tab still open")
	}
	m = openSelected(t, m)
	if strings.Contains(m.files.input.Value(), "unsaved") {
		t.Fatal("discarded edits survived reopen")
	}
}

func TestEditorSwitchBlockedWhileSaving(t *testing.T) {
	m, _ := twoFileFixture(t)
	m = openSelected(t, m)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	next, save := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(Model)
	if save == nil {
		t.Fatal("save was not scheduled")
	}
	path := m.files.document.Path
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = next.(Model)
	if cmd != nil || m.files.document.Path != path {
		t.Fatal("tab switch allowed during save")
	}
	next, _ = m.Update(save())
	m = next.(Model)
}

func TestEditorQuitRefusedWithBackgroundDirtyTab(t *testing.T) {
	m, _ := twoFileFixture(t)
	m = openSelected(t, m)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	m = openSelected(t, m)
	next, _ = m.Update(key("dirty-tab"))
	m = next.(Model)
	// Stash the dirty tab by switching back to the clean one.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m = next.(Model)
	if m.files.dirty() {
		t.Fatal("expected to switch to the clean tab")
	}
	next, quit := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(Model)
	if quit != nil {
		t.Fatal("quit allowed with dirty background tab")
	}
	if !strings.Contains(m.Status, "Unsaved changes") {
		t.Fatalf("status = %q", m.Status)
	}
}
