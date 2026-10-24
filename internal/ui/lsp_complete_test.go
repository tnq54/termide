package ui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
	"termide/internal/protocol"
)

func completeFixture() (Model, *[]protocol.Message) {
	var sent []protocol.Message
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		sent = append(sent, request)
		payload, _ := json.Marshal(protocol.LspCompleteResult{Items: []protocol.LspCompleteItem{
			{Label: "append", Kind: 2, Detail: "list.append"},
			{Label: "abs", Kind: 3, Detail: "builtin"},
		}})
		return protocol.Message{Type: protocol.MessageLspCompleteResult, WorkspaceID: "w", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x = [].a"}
	m.files.input.SetValue("x = [].a")
	return m, &sent
}

func TestCompleteShowsPopup(t *testing.T) {
	m, sent := completeFixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("completion not scheduled")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if len(m.completeItems) != 2 {
		t.Fatalf("items = %#v", m.completeItems)
	}
	if len(*sent) != 1 || (*sent)[0].Type != protocol.MessageLspComplete {
		t.Fatalf("sent = %#v", sent)
	}
	var request protocol.LspCompleteRequest
	if err := json.Unmarshal((*sent)[0].Payload, &request); err != nil || request.Path != "x.py" {
		t.Fatalf("request = %#v", request)
	}
	if !strings.Contains(m.View(), "append") {
		t.Fatalf("view = %q", m.View())
	}
}

func TestCompleteAcceptInsertsWord(t *testing.T) {
	m, _ := completeFixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if len(m.completeItems) != 0 {
		t.Fatal("popup not closed after accept")
	}
	if !strings.Contains(m.files.input.Value(), "x = [].abs") {
		t.Fatalf("value = %q", m.files.input.Value())
	}
}

func TestCompleteEscapeClosesPopupNotTab(t *testing.T) {
	m, _ := completeFixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if len(m.completeItems) != 0 {
		t.Fatal("popup not closed")
	}
	if m.files.document == nil {
		t.Fatal("escape closed the tab instead of the popup")
	}
}

func TestCompleteRejectsForeignResponse(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		payload, _ := json.Marshal(protocol.LspCompleteResult{Items: []protocol.LspCompleteItem{{Label: "x"}}})
		return protocol.Message{Type: protocol.MessageLspCompleteResult, WorkspaceID: "other", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x\n"}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if len(m.completeItems) != 0 {
		t.Fatal("foreign completion accepted")
	}
}

func TestCompleteEmptyShowsStatus(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		payload, _ := json.Marshal(protocol.LspCompleteResult{})
		return protocol.Message{Type: protocol.MessageLspCompleteResult, WorkspaceID: "w", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x\n"}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if len(m.completeItems) != 0 || m.Status == "" {
		t.Fatal("empty completion not reported")
	}
}
