package ui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
	"termide/internal/protocol"
)

func codeActionFixture() (Model, *[]protocol.Message) {
	var sent []protocol.Message
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		sent = append(sent, request)
		switch request.Type {
		case protocol.MessageLspCodeAction:
			payload, _ := json.Marshal(protocol.LspCodeActionList{Actions: []protocol.LspCodeActionItem{{ID: "act-1-0", Title: "Add missing import", Kind: "quickfix"}}})
			return protocol.Message{Type: protocol.MessageLspCodeActionList, WorkspaceID: "w", Payload: payload}, nil
		case protocol.MessageLspCodeActionApply:
			payload, _ := json.Marshal(protocol.LspRenameResult{Files: []protocol.LspRenameFile{{Path: "x.py", Edits: 1}}})
			return protocol.Message{Type: protocol.MessageLspCodeActionApplied, WorkspaceID: "w", Payload: payload}, nil
		default:
			payload, _ := json.Marshal(protocol.FileResponse{Path: "x.py", Document: &domain.FileDocument{Path: "x.py", Text: "import os\nx = foo\n"}})
			return protocol.Message{Type: protocol.MessageFileResult, WorkspaceID: "w", Payload: payload}, nil
		}
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x = foo"}
	m.files.input.SetValue("x = foo")
	return m, &sent
}

func TestCodeActionListsAndApplies(t *testing.T) {
	m, sent := codeActionFixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("code action list not scheduled")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if len(m.codeActions) != 1 {
		t.Fatalf("actions = %#v", m.codeActions)
	}
	if (*sent)[0].Type != protocol.MessageLspCodeAction {
		t.Fatalf("sent = %#v", sent)
	}
	var request protocol.LspCodeActionRequest
	if err := json.Unmarshal((*sent)[0].Payload, &request); err != nil {
		t.Fatal(err)
	}
	if request.Path != "x.py" || request.StartLine != 0 || request.EndChar <= 0 {
		t.Fatalf("request = %#v", request)
	}
	if !strings.Contains(m.View(), "Add missing import") {
		t.Fatalf("view = %q", m.View())
	}
	next, apply := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if apply == nil {
		t.Fatal("apply not scheduled")
	}
	if len(m.codeActions) != 0 {
		t.Fatal("popup not closed on accept")
	}
	next, reload := m.Update(apply())
	m = next.(Model)
	if reload == nil {
		t.Fatal("open document not refreshed after apply")
	}
	found := false
	for _, message := range *sent {
		if message.Type != protocol.MessageLspCodeActionApply {
			continue
		}
		var applyRequest protocol.LspCodeActionApplyRequest
		if err := json.Unmarshal(message.Payload, &applyRequest); err != nil {
			t.Fatal(err)
		}
		if applyRequest.ID != "act-1-0" {
			t.Fatalf("apply = %#v", applyRequest)
		}
		found = true
	}
	if !found {
		t.Fatalf("sent = %#v", sent)
	}
	next, _ = m.Update(reload())
	m = next.(Model)
	if !strings.Contains(m.Status, "1 file") {
		t.Fatalf("status = %q", m.Status)
	}
}

func TestCodeActionEscapeClosesPopup(t *testing.T) {
	m, _ := codeActionFixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
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

func TestCodeActionDirtyBufferWarnsInsteadOfReload(t *testing.T) {
	m, _ := codeActionFixture()
	next, _ := m.Update(key(" edited"))
	m = next.(Model)
	if !m.files.dirty() {
		t.Fatal("test setup broken: buffer should be dirty")
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	next, apply := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if apply == nil {
		t.Fatal("apply not scheduled")
	}
	next, reload := m.Update(apply())
	m = next.(Model)
	if reload != nil {
		t.Fatal("dirty buffer would be overwritten by reload")
	}
	if !strings.Contains(m.Status, "unsaved") {
		t.Fatalf("status = %q", m.Status)
	}
}

func TestCodeActionRejectsForeignResponse(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		payload, _ := json.Marshal(protocol.LspCodeActionList{})
		return protocol.Message{Type: protocol.MessageLspCodeActionList, WorkspaceID: "other", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x\n"}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if len(m.codeActions) != 0 {
		t.Fatal("foreign response accepted")
	}
}

func TestCodeActionEmptyShowsStatus(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		payload, _ := json.Marshal(protocol.LspCodeActionList{})
		return protocol.Message{Type: protocol.MessageLspCodeActionList, WorkspaceID: "w", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x\n"}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if len(m.codeActions) != 0 || m.Status == "" {
		t.Fatal("empty list not reported")
	}
}
