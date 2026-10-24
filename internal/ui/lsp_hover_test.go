package ui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
	"termide/internal/protocol"
)

func hoverFixture() (Model, *[]protocol.Message) {
	var sent []protocol.Message
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		sent = append(sent, request)
		payload, _ := json.Marshal(protocol.LspHoverResult{Contents: "**len** builtin"})
		return protocol.Message{Type: protocol.MessageLspHoverResult, WorkspaceID: "w", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x = len([1])\n"}
	m.files.input.SetValue("x = len([1])\n")
	return m, &sent
}

func TestHoverShowsPopup(t *testing.T) {
	m, sent := hoverFixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("hover not scheduled")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.hover == "" {
		t.Fatal("hover popup empty")
	}
	if len(*sent) != 1 || (*sent)[0].Type != protocol.MessageLspHover {
		t.Fatalf("sent = %#v", sent)
	}
	var request protocol.LspHoverRequest
	if err := json.Unmarshal((*sent)[0].Payload, &request); err != nil || request.Path != "x.py" {
		t.Fatalf("request = %#v", request)
	}
	if !strings.Contains(m.View(), "len") {
		t.Fatalf("view = %q", m.View())
	}
}

func TestHoverEscapeClosesPopupNotTab(t *testing.T) {
	m, _ := hoverFixture()
	m.hover = "popup"
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.hover != "" {
		t.Fatal("popup not closed")
	}
	if m.files.document == nil {
		t.Fatal("escape closed the tab instead of the popup")
	}
}

func TestHoverRejectsForeignResponse(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		payload, _ := json.Marshal(protocol.LspHoverResult{Contents: "x"})
		return protocol.Message{Type: protocol.MessageLspHoverResult, WorkspaceID: "other", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x\n"}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.hover != "" {
		t.Fatal("foreign hover accepted")
	}
}

func TestHoverErrorShowsStatus(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		return protocol.Message{}, errHoverBoom{}
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x\n"}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.hover != "" || m.Status == "" {
		t.Fatal("error not reported")
	}
}

type errHoverBoom struct{}

func (errHoverBoom) Error() string { return "connection lost" }
