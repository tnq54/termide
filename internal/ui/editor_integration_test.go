package ui

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"termide/internal/client"
	"termide/internal/domain"
	"termide/internal/server"
	"termide/internal/store"
)

func editorFixture(t *testing.T) (Model, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.py"), []byte("pass\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	done := make(chan error, 1)
	go func() { defer right.Close(); done <- server.NewHandler(s).Serve(context.Background(), right, right) }()
	remote := client.NewRemote(left)
	t.Cleanup(func() { remote.Close(); <-done; s.Close() })
	m := NewRemoteModel(remote, domain.Workspace{ID: "w", Root: root})
	next, _ := m.Update(m.Init()())
	m = next.(Model)
	next, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 18})
	m = next.(Model)
	return m, root
}

func applyCommand(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatalf("expected remote command for %v", msg)
	}
	next, _ = next.Update(cmd())
	return next.(Model)
}

func TestEditorBrowseEditSaveThroughServer(t *testing.T) {
	m, root := editorFixture(t)
	m = applyCommand(t, m, key("e"))
	if len(m.files.entries) != 1 {
		t.Fatal("file browser missing entries")
	}
	m = applyCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.files.document == nil {
		t.Fatal("editor did not open file")
	}
	next, cmd := m.Update(key("q tiếng Việt"))
	m = next.(Model)
	_ = cmd // textarea blink commands do not perform I/O
	if !m.files.dirty() {
		t.Fatal("edit did not mark buffer dirty")
	}
	edited := m.files.input.Value()
	if !strings.Contains(edited, "q tiếng Việt") {
		t.Fatal(edited)
	}
	m = applyCommand(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	data, err := os.ReadFile(filepath.Join(root, "main.py"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != edited || m.files.dirty() {
		t.Fatalf("saved %q, editor %q", data, edited)
	}
}

func TestEditorConflictKeepsUnsavedText(t *testing.T) {
	m, root := editorFixture(t)
	m = applyCommand(t, m, key("e"))
	m = applyCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	next, _ := m.Update(key("local edit"))
	m = next.(Model)
	edited := m.files.input.Value()
	os.WriteFile(filepath.Join(root, "main.py"), []byte("external edit"), 0600)
	m = applyCommand(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if !m.files.dirty() || m.files.input.Value() != edited || !strings.Contains(m.Status, "changed on server") {
		t.Fatal(m.Status)
	}
	data, _ := os.ReadFile(filepath.Join(root, "main.py"))
	if string(data) != "external edit" {
		t.Fatal("overwrote remote edit")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.files.document == nil {
		t.Fatal("escape silently discarded edits")
	}
}

func TestEditorUndoRedoUnicode(t *testing.T) {
	m, _ := editorFixture(t)
	m = applyCommand(t, m, key("e"))
	m = applyCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	before := m.files.input.Value()
	next, _ := m.Update(key("sửa mã"))
	m = next.(Model)
	after := m.files.input.Value()
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	m = next.(Model)
	if m.files.input.Value() != before {
		t.Fatal("undo mismatch")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	m = next.(Model)
	if m.files.input.Value() != after {
		t.Fatal("redo mismatch")
	}
}

func TestEditorSaveAckKeepsNewerEditsDirty(t *testing.T) {
	m, root := editorFixture(t)
	m = applyCommand(t, m, key("e"))
	m = applyCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	next, _ := m.Update(key("first"))
	m = next.(Model)
	snapshot := m.files.input.Value()
	next, save := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(Model)
	next, _ = m.Update(key("second"))
	m = next.(Model)
	next, _ = m.Update(save())
	m = next.(Model)
	if !m.files.dirty() || !strings.Contains(m.files.input.Value(), "second") {
		t.Fatal("save acknowledgment discarded newer edits")
	}
	data, _ := os.ReadFile(filepath.Join(root, "main.py"))
	if string(data) != snapshot {
		t.Fatal("wrong snapshot saved")
	}
}

func TestEditorViewFitsPhoneAndPreservesCRLF(t *testing.T) {
	m, root := editorFixture(t)
	original := "def hello():\r\n\tprint('xin chào')\r\n"
	if err := os.WriteFile(filepath.Join(root, "main.py"), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	m = applyCommand(t, m, key("e"))
	m = applyCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.files.input.Value() != original || m.files.dirty() {
		t.Fatal("source changed on open")
	}
	for _, size := range [][2]int{{40, 12}, {24, 8}, {1, 1}, {120, 35}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(Model)
		lines := strings.Split(m.View(), "\n")
		if len(lines) > size[1] {
			t.Fatal("too many rows")
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("line wider than terminal")
			}
		}
	}
}

func TestEditorCannotQuitDuringSaveAfterUndo(t *testing.T) {
	m, _ := editorFixture(t)
	m = applyCommand(t, m, key("e"))
	m = applyCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	next, _ := m.Update(key("edit"))
	m = next.(Model)
	next, save := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	m = next.(Model)
	if m.files.dirty() {
		t.Fatal("expected undo to restore original text")
	}
	next, quit := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(Model)
	if quit != nil {
		t.Fatal("quit allowed while snapshot is being saved")
	}
	next, _ = m.Update(save())
	m = next.(Model)
	if !m.files.dirty() {
		t.Fatal("undo should remain unsaved after snapshot ack")
	}
}
