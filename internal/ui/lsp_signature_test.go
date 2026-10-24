package ui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
	"termide/internal/protocol"
)

func signatureFixture() (Model, *[]protocol.Message) {
	var sent []protocol.Message
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		sent = append(sent, request)
		payload, _ := json.Marshal(protocol.LspSignatureResult{Signatures: []protocol.LspSignature{
			{Label: "len(obj, /)", Documentation: "Return the number of items.", Parameters: []protocol.LspSignatureParameter{{Label: "obj"}}},
		}, ActiveSignature: 0, ActiveParameter: 0})
		return protocol.Message{Type: protocol.MessageLspSignatureResult, WorkspaceID: "w", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x = len"}
	m.files.input.SetValue("x = len")
	return m, &sent
}

func TestSignatureAutoShowsOnParen(t *testing.T) {
	m, sent := signatureFixture()
	next, cmd := m.Update(key("("))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("signature help not scheduled on paren")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.sigHelp == nil {
		t.Fatal("signature popup did not open")
	}
	if len(*sent) != 1 || (*sent)[0].Type != protocol.MessageLspSignature {
		t.Fatalf("sent = %#v", sent)
	}
}

func TestSignatureManualTrigger(t *testing.T) {
	m, sent := signatureFixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("signature help not scheduled")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if len(*sent) != 1 || (*sent)[0].Type != protocol.MessageLspSignature {
		t.Fatalf("sent = %#v", sent)
	}
	var request protocol.LspSignatureRequest
	if err := json.Unmarshal((*sent)[0].Payload, &request); err != nil || request.Path != "x.py" {
		t.Fatalf("request = %#v", request)
	}
	if m.sigHelp == nil || !strings.Contains(m.View(), "len(obj") {
		t.Fatalf("view = %q", m.View())
	}
}

func TestSignatureEscapeClosesPopupNotTab(t *testing.T) {
	m, _ := signatureFixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.sigHelp == nil {
		t.Fatal("popup did not open")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.sigHelp != nil {
		t.Fatal("popup not closed")
	}
	if m.files.document == nil {
		t.Fatal("escape closed the tab instead of the popup")
	}
}

func TestSignatureCyclesOverloads(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		payload, _ := json.Marshal(protocol.LspSignatureResult{Signatures: []protocol.LspSignature{
			{Label: "f(a)"}, {Label: "f(a, b)"},
		}})
		return protocol.Message{Type: protocol.MessageLspSignatureResult, WorkspaceID: "w", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "f("}
	m.files.input.SetValue("f(")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.sigSel != 0 {
		t.Fatalf("sigSel = %d", m.sigSel)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.sigSel != 1 {
		t.Fatalf("sigSel = %d", m.sigSel)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(Model)
	if m.sigSel != 0 {
		t.Fatalf("sigSel = %d", m.sigSel)
	}
}

func TestSignatureParenClosesPopup(t *testing.T) {
	m, _ := signatureFixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.sigHelp == nil {
		t.Fatal("popup did not open")
	}
	next, _ = m.Update(key(")"))
	m = next.(Model)
	if m.sigHelp != nil {
		t.Fatal("popup not closed on paren")
	}
	if !strings.Contains(m.files.input.Value(), ")") {
		t.Fatalf("paren not inserted: %q", m.files.input.Value())
	}
}

func TestSignatureAutoSuppressedAfterError(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		return protocol.Message{}, errSigBoom{}
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "f"}
	m.files.input.SetValue("f")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.Status == "" {
		t.Fatal("error not reported")
	}
	before := len(m.files.input.Value())
	next, auto := m.Update(key("("))
	m = next.(Model)
	if auto != nil {
		t.Fatal("auto request not suppressed after error")
	}
	if len(m.files.input.Value()) != before+1 {
		t.Fatal("paren not inserted")
	}
}

type errSigBoom struct{}

func (errSigBoom) Error() string { return "connection lost" }

func TestSignatureRejectsForeignResponse(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		payload, _ := json.Marshal(protocol.LspSignatureResult{Signatures: []protocol.LspSignature{{Label: "f()"}}})
		return protocol.Message{Type: protocol.MessageLspSignatureResult, WorkspaceID: "other", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x\n"}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.sigHelp != nil {
		t.Fatal("foreign signature accepted")
	}
}
