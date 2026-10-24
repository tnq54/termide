package ui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
	"termide/internal/protocol"
)

func sourceActionFixture() (Model, *[]protocol.Message) {
	var sent []protocol.Message
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		sent = append(sent, request)
		switch request.Type {
		case protocol.MessageLspCodeAction:
			payload, _ := json.Marshal(protocol.LspCodeActionList{Actions: []protocol.LspCodeActionItem{{ID: "act-9", Title: "Organize imports", Kind: "source.organizeImports"}}})
			return protocol.Message{Type: protocol.MessageLspCodeActionList, WorkspaceID: "w", Payload: payload}, nil
		default:
			payload, _ := json.Marshal(protocol.LspRenameResult{Files: []protocol.LspRenameFile{{Path: "x.py", Edits: 1}}})
			return protocol.Message{Type: protocol.MessageLspCodeActionApplied, WorkspaceID: "w", Payload: payload}, nil
		}
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "import b\nimport a\n"}
	m.files.input.SetValue("import b\nimport a\n")
	return m, &sent
}

func TestSourceActionListsWholeFileActions(t *testing.T) {
	m, sent := sourceActionFixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("source action not scheduled")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if len(m.codeActions) != 1 {
		t.Fatalf("actions = %#v", m.codeActions)
	}
	var request protocol.LspCodeActionRequest
	if err := json.Unmarshal((*sent)[0].Payload, &request); err != nil {
		t.Fatal(err)
	}
	if len(request.Only) != 1 || request.Only[0] != "source" {
		t.Fatalf("request = %#v", request)
	}
	if request.StartLine != 0 || request.StartChar != 0 || request.EndLine != 2 {
		t.Fatalf("request range not whole file: %#v", request)
	}
	if !strings.Contains(m.View(), "Organize imports") {
		t.Fatalf("view = %q", m.View())
	}
}

func TestSourceActionAppliesAndReloads(t *testing.T) {
	m, sent := sourceActionFixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	next, apply := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if apply == nil {
		t.Fatal("apply not scheduled")
	}
	_ = sent
	next, reload := m.Update(apply())
	m = next.(Model)
	if reload == nil {
		t.Fatal("open document not refreshed after apply")
	}
	next, _ = m.Update(reload())
	_ = next
	m = next.(Model)
	if m.Status == "" {
		t.Fatal("status not set")
	}
}

func TestSourceActionEscapeClosesPopup(t *testing.T) {
	m, _ := sourceActionFixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if len(m.codeActions) == 0 {
		t.Fatal("popup did not open")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if len(m.codeActions) != 0 {
		t.Fatal("popup not closed")
	}
	if m.files.document == nil {
		t.Fatal("escape closed the tab instead of the popup")
	}
}
