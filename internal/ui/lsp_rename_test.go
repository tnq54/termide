package ui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
	"termide/internal/protocol"
)

func renameFixture() (Model, *[]protocol.Message) {
	var sent []protocol.Message
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		sent = append(sent, request)
		switch request.Type {
		case protocol.MessageLspRename:
			payload, _ := json.Marshal(protocol.LspRenameResult{Files: []protocol.LspRenameFile{{Path: "x.py", Edits: 1}}})
			return protocol.Message{Type: protocol.MessageLspRenameResult, WorkspaceID: "w", Payload: payload}, nil
		default:
			payload, _ := json.Marshal(protocol.FileResponse{Path: "x.py", Document: &domain.FileDocument{Path: "x.py", Text: "x = total\n"}})
			return protocol.Message{Type: protocol.MessageFileResult, WorkspaceID: "w", Payload: payload}, nil
		}
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x = sum\n"}
	m.files.input.SetValue("x = sum\n")
	return m, &sent
}

func TestRenamePromptCollectsName(t *testing.T) {
	m, sent := renameFixture()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(Model)
	if !m.renaming {
		t.Fatal("rename mode not entered")
	}
	for _, key := range []tea.KeyMsg{key("t"), key("o"), key("t"), key("a"), key("l")} {
		next, _ = m.Update(key)
		m = next.(Model)
	}
	if m.renameBuf != "total" {
		t.Fatalf("buffer = %q", m.renameBuf)
	}
	if len(*sent) != 0 {
		t.Fatalf("request sent before Enter: %#v", sent)
	}
}

func TestRenameSubmitRefreshesOpenDoc(t *testing.T) {
	m, sent := renameFixture()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(Model)
	for _, key := range []tea.KeyMsg{key("t"), key("o"), key("t"), key("a"), key("l")} {
		next, _ = m.Update(key)
		m = next.(Model)
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.renaming {
		t.Fatal("rename mode not exited")
	}
	if cmd == nil {
		t.Fatal("rename not scheduled")
	}
	next, reload := m.Update(cmd())
	m = next.(Model)
	if len(*sent) == 0 || (*sent)[0].Type != protocol.MessageLspRename {
		t.Fatalf("sent = %#v", sent)
	}
	var request protocol.LspRenameRequest
	if err := json.Unmarshal((*sent)[0].Payload, &request); err != nil {
		t.Fatal(err)
	}
	if request.Path != "x.py" || request.NewName != "total" {
		t.Fatalf("request = %#v", request)
	}
	if reload == nil {
		t.Fatal("open document not refreshed after rename")
	}
	next, _ = m.Update(reload())
	m = next.(Model)
	if m.files.input.Value() != "x = total\n" {
		t.Fatalf("buffer = %q", m.files.input.Value())
	}
	if !strings.Contains(m.Status, "1 file") {
		t.Fatalf("status = %q", m.Status)
	}
}

func TestRenameEscapeCancels(t *testing.T) {
	m, sent := renameFixture()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(Model)
	next, _ = m.Update(key("x"))
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.renaming {
		t.Fatal("rename mode not cancelled")
	}
	if len(*sent) != 0 {
		t.Fatalf("request sent after cancel: %#v", sent)
	}
	if m.files.document == nil {
		t.Fatal("escape closed the tab instead of cancelling rename")
	}
}

func TestRenameDirtyBufferWarnsInsteadOfReload(t *testing.T) {
	m, sent := renameFixture()
	next, _ := m.Update(key(" edited"))
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(Model)
	for _, key := range []tea.KeyMsg{key("t")} {
		next, _ = m.Update(key)
		m = next.(Model)
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("rename not scheduled")
	}
	next, reload := m.Update(cmd())
	m = next.(Model)
	if reload != nil {
		t.Fatal("dirty buffer should not auto-reload")
	}
	if !strings.Contains(m.Status, "unsaved") {
		t.Fatalf("status = %q", m.Status)
	}
	for _, request := range *sent {
		if request.Type == protocol.MessageReadFile {
			t.Fatal("dirty buffer reloaded, edits would be lost")
		}
	}
}

func TestRenameRejectsForeignResponse(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		payload, _ := json.Marshal(protocol.LspRenameResult{Files: []protocol.LspRenameFile{{Path: "x.py", Edits: 1}}})
		return protocol.Message{Type: protocol.MessageLspRenameResult, WorkspaceID: "other", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x = sum\n"}
	m.renaming = true
	m.renameBuf = "total"
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.renaming {
		t.Fatal("rename mode stuck")
	}
	_ = cmd
}
