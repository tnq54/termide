package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"termide/internal/domain"
	"termide/internal/editor"
	"termide/internal/protocol"
)

type editorTab struct {
	document *domain.FileDocument
	input    editor.Input
	undo     []string
	redo     []string
}

type fileEditor struct {
	folder   string
	entries  []domain.FileEntry
	selected int
	// document/input/undo/redo are the active tab's working state.
	// Inactive tabs rest in tabs; the file list shows when document is nil.
	document   *domain.FileDocument
	input      editor.Input
	undo, redo []string
	tabs       []editorTab
	width      int
	height     int
	busy       bool
}

func newFileEditor() fileEditor {
	input := editor.NewInput()
	input.SetWidth(40)
	input.SetHeight(10)
	return fileEditor{folder: ".", input: input, width: 40, height: 10}
}
func (f fileEditor) dirty() bool { return f.document != nil && f.input.Value() != f.document.Text }

func (f fileEditor) anyDirty() bool {
	if f.dirty() {
		return true
	}
	for _, tab := range f.tabs {
		if tab.document != nil && tab.input.Value() != tab.document.Text {
			return true
		}
	}
	return false
}

// stashActive parks the active tab into the background list, replacing any
// entry for the same path so reopening never duplicates a tab.
func (f *fileEditor) stashActive() {
	if f.document == nil {
		return
	}
	tab := editorTab{document: f.document, input: f.input, undo: f.undo, redo: f.redo}
	for i := range f.tabs {
		if f.tabs[i].document != nil && f.tabs[i].document.Path == f.document.Path {
			f.tabs[i] = tab
			f.document = nil
			return
		}
	}
	f.tabs = append(f.tabs, tab)
	f.document = nil
}

func (f *fileEditor) loadTab(index int) {
	tab := f.tabs[index]
	f.tabs = append(f.tabs[:index], f.tabs[index+1:]...)
	f.document = tab.document
	f.input = tab.input
	f.undo = tab.undo
	f.redo = tab.redo
}

// discardActive drops the active tab without stashing it.
func (f *fileEditor) discardActive() {
	f.document = nil
	if len(f.tabs) == 0 {
		return
	}
	last := f.tabs[len(f.tabs)-1]
	f.tabs = f.tabs[:len(f.tabs)-1]
	f.document = last.document
	f.input = last.input
	f.undo = last.undo
	f.redo = last.redo
}

// rotateNext stashes the active tab at the end and loads the oldest one.
func (f *fileEditor) rotateNext() {
	active := editorTab{document: f.document, input: f.input, undo: f.undo, redo: f.redo}
	f.tabs = append(f.tabs, active)
	next := f.tabs[0]
	f.tabs = f.tabs[1:]
	f.document = next.document
	f.input = next.input
	f.undo = next.undo
	f.redo = next.redo
}

// rotatePrev stashes the active tab at the front and loads the newest one.
func (f *fileEditor) rotatePrev() {
	active := editorTab{document: f.document, input: f.input, undo: f.undo, redo: f.redo}
	f.tabs = append([]editorTab{active}, f.tabs...)
	last := len(f.tabs) - 1
	next := f.tabs[last]
	f.tabs = f.tabs[:last]
	f.document = next.document
	f.input = next.input
	f.undo = next.undo
	f.redo = next.redo
}

func (f *fileEditor) newTabInput() editor.Input {
	input := editor.NewInput()
	input.SetWidth(max(1, f.width))
	input.SetHeight(max(1, f.height))
	return input
}

func (f *fileEditor) tabBar() string {
	var names []string
	for _, tab := range f.tabs {
		if tab.document == nil {
			continue
		}
		name := safeLabel(tab.document.Path)
		if tab.input.Value() != tab.document.Text {
			name += "*"
		}
		names = append(names, "["+name+"]")
	}
	if f.document != nil {
		name := safeLabel(f.document.Path)
		if f.input.Value() != f.document.Text {
			name += "*"
		}
		names = append(names, ">"+name+"<")
	}
	if len(names) == 0 {
		return ""
	}
	return "tabs: " + strings.Join(names, " ")
}

type fileResult struct {
	kind     protocol.MessageType
	response protocol.FileResponse
	err      error
}

func (m Model) fileCommand(kind protocol.MessageType, request protocol.FileRequest) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		payload, err := json.Marshal(request)
		if err != nil {
			return fileResult{kind: kind, err: err}
		}
		response, err := m.remote.Call(ctx, protocol.Message{Type: kind, WorkspaceID: m.workspace.ID, Payload: payload})
		if err != nil {
			return fileResult{kind: kind, err: err}
		}
		result := fileResult{kind: kind}
		if response.Type != protocol.MessageFileResult || response.WorkspaceID != m.workspace.ID {
			return fileResult{kind: kind, err: fmt.Errorf("unexpected file response")}
		}
		if err := json.Unmarshal(response.Payload, &result.response); err != nil {
			return fileResult{kind: kind, err: err}
		}
		if result.response.Path != request.Path {
			return fileResult{kind: kind, err: fmt.Errorf("file response path mismatch")}
		}
		if kind != protocol.MessageListFiles && (result.response.Document == nil || result.response.Document.Path != request.Path) {
			return fileResult{kind: kind, err: fmt.Errorf("missing file document")}
		}
		return result
	}
}
func (m Model) receiveFile(result fileResult) (tea.Model, tea.Cmd) {
	m.files.busy = false
	if result.err != nil {
		m.Status = result.err.Error()
		return m, nil
	}
	switch result.kind {
	case protocol.MessageListFiles:
		m.files.folder = result.response.Path
		m.files.entries = result.response.Entries
		m.files.selected = 0
		m.Status = "files loaded"
	case protocol.MessageReadFile:
		m.files.document = result.response.Document
		m.files.input.SetValue(m.files.document.Text)
		m.files.undo = nil
		m.files.redo = nil
		m.Status = "file opened"
		return m, m.files.input.Focus()
	case protocol.MessageSaveFile:
		// Save acknowledges a snapshot. Edits made during the request remain dirty.
		m.files.document = result.response.Document
		m.Status = "file saved"
	}
	return m, nil
}

func (m Model) editorKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.files.document != nil {
		switch key.Type {
		case tea.KeyEsc:
			if m.hover != "" {
				m.hover = ""
				return m, nil
			}
			if m.files.busy {
				return m, nil
			}
			if m.files.dirty() {
				m.Status = "Unsaved changes: Ctrl+S save; Ctrl+G discard"
				return m, nil
			}
			m.files.stashActive()
			m.files.input.Blur()
			return m, nil
		case tea.KeyCtrlK:
			if m.remote == nil || !m.ready || m.files.busy {
				return m, nil
			}
			line, col := m.files.input.CursorLineCol()
			return m, m.hoverCommand(m.files.document.Path, line, col)
		case tea.KeyCtrlG:
			if m.files.busy {
				return m, nil
			}
			m.files.discardActive()
			m.files.input.Blur()
			m.Status = "edits discarded"
			return m, nil
		case tea.KeyCtrlN:
			if m.files.busy || len(m.files.tabs) == 0 {
				return m, nil
			}
			m.files.rotateNext()
			m.Status = "next tab"
			return m, nil
		case tea.KeyCtrlP:
			if m.files.busy || len(m.files.tabs) == 0 {
				return m, nil
			}
			m.files.rotatePrev()
			m.Status = "previous tab"
			return m, nil
		case tea.KeyCtrlS:
			if m.files.busy || !m.files.dirty() {
				return m, nil
			}
			m.files.busy = true
			m.Status = "saving file"
			return m, m.fileCommand(protocol.MessageSaveFile, protocol.FileRequest{Path: m.files.document.Path, Version: m.files.document.Version, Text: m.files.input.Value()})
		case tea.KeyCtrlZ:
			if n := len(m.files.undo); n > 0 {
				m.files.redo = append(m.files.redo, m.files.input.Value())
				m.files.input.SetValue(m.files.undo[n-1])
				m.files.undo = m.files.undo[:n-1]
			}
			return m, nil
		case tea.KeyCtrlY:
			if n := len(m.files.redo); n > 0 {
				m.files.undo = append(m.files.undo, m.files.input.Value())
				m.files.input.SetValue(m.files.redo[n-1])
				m.files.redo = m.files.redo[:n-1]
			}
			return m, nil
		}
		before := m.files.input.Value()
		var cmd tea.Cmd
		m.files.input, cmd = m.files.input.Update(key)
		if m.files.input.Value() != before {
			m.hover = ""
			m.files.undo = append(m.files.undo, before)
			m.files.redo = nil
			if len(m.files.undo) > 50 {
				m.files.undo = m.files.undo[1:]
			}
		}
		return m, cmd
	}
	if m.files.busy {
		return m, nil
	}
	switch key.Type {
	case tea.KeyEsc:
		m.Panel = PanelDashboard
	case tea.KeyUp:
		if m.files.selected > 0 {
			m.files.selected--
		}
	case tea.KeyDown:
		if m.files.selected+1 < len(m.files.entries) {
			m.files.selected++
		}
	case tea.KeyBackspace, tea.KeyCtrlH:
		if m.remote != nil && m.ready && m.files.folder != "." {
			m.files.busy = true
			return m, m.fileCommand(protocol.MessageListFiles, protocol.FileRequest{Path: path.Dir(m.files.folder)})
		}
	case tea.KeyEnter:
		if m.remote == nil || !m.ready || len(m.files.entries) == 0 {
			return m, nil
		}
		entry := m.files.entries[m.files.selected]
		kind := protocol.MessageReadFile
		if entry.Directory {
			kind = protocol.MessageListFiles
		}
		target := path.Join(m.files.folder, entry.Name)
		if kind == protocol.MessageReadFile {
			if m.files.document != nil && m.files.document.Path == target {
				return m, m.files.input.Focus()
			}
			for i := range m.files.tabs {
				if m.files.tabs[i].document != nil && m.files.tabs[i].document.Path == target {
					if m.files.busy {
						return m, nil
					}
					m.files.stashActive()
					m.files.loadTab(i)
					return m, m.files.input.Focus()
				}
			}
			m.files.stashActive()
			m.files.input = m.files.newTabInput()
		}
		m.files.busy = true
		m.Status = "loading"
		return m, m.fileCommand(kind, protocol.FileRequest{Path: target})
	}
	return m, nil
}

func (m Model) editorView() string {
	width, height := m.Width, m.Height
	if width < 1 {
		width = 40
	}
	if height < 1 {
		height = 18
	}
	var lines []string
	if bar := m.files.tabBar(); bar != "" {
		lines = append(lines, bar)
	}
	if m.files.document != nil {
		marker := ""
		if m.files.dirty() {
			marker = " *"
		}
		lines = append(lines, safeLabel(m.files.document.Path)+marker)
		lines = append(lines, strings.Split(m.files.input.View(), "\n")...)
		lines = renderHover(lines, m.hover, width)
		lines = append(lines, "Ctrl+S save | Esc files | Ctrl+N/P tabs | Ctrl+K hover", "Ctrl+Z/Y undo/redo | Ctrl+G discard")
	} else {
		lines = append(lines, "Files: "+safeLabel(m.files.folder))
		visible := max(1, height-4)
		start := max(0, m.files.selected-visible+1)
		for i := start; i < len(m.files.entries) && i < start+visible; i++ {
			entry := m.files.entries[i]
			prefix := "  "
			if i == m.files.selected {
				prefix = "> "
			}
			suffix := ""
			if entry.Directory {
				suffix = "/"
			}
			lines = append(lines, prefix+safeLabel(entry.Name)+suffix)
		}
		if len(m.files.entries) == 0 {
			lines = append(lines, "(empty directory)")
		}
		lines = append(lines, "Enter open | Backspace parent", "Esc dashboard")
	}
	lines = append(lines, safeLabel(m.Status))
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "")
	}
	return strings.Join(lines, "\n")
}
func safeLabel(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || r >= 128 && r <= 159 {
			return ' '
		}
		return r
	}, value)
}
