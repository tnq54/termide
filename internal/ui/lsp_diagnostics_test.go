package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
)

func diagnosticsFixture() (Model, *[]protocol.Message) {
	var sent []protocol.Message
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		sent = append(sent, request)
		payload, _ := json.Marshal(protocol.LspDiagnosticsResult{Items: []protocol.LspDiagnostic{{Line: 1, Character: 4, Message: "undefined name 'foo'", Severity: 1}}})
		return protocol.Message{Type: protocol.MessageLspDiagnosticsResult, WorkspaceID: "w", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x = 1\ny = foo\n"}
	return m, &sent
}

func TestDiagnosticsFetchedAndRendered(t *testing.T) {
	m, sent := diagnosticsFixture()
	next, _ := m.Update(m.diagnosticsCommand("x.py")())
	m = next.(Model)
	if len(*sent) != 1 || (*sent)[0].Type != protocol.MessageLspDiagnostics {
		t.Fatalf("sent = %#v", sent)
	}
	if len(m.diags) != 1 || !strings.Contains(m.View(), "undefined name") {
		t.Fatalf("view = %q", m.View())
	}
}

func TestDiagnosticsRejectsForeignResponse(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		payload, _ := json.Marshal(protocol.LspDiagnosticsResult{Items: []protocol.LspDiagnostic{{Line: 0, Message: "x"}}})
		return protocol.Message{Type: protocol.MessageLspDiagnosticsResult, WorkspaceID: "other", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x\n"}
	next, _ := m.Update(m.diagnosticsCommand("x.py")())
	m = next.(Model)
	if len(m.diags) != 0 {
		t.Fatal("foreign diagnostics accepted")
	}
}

func TestDiagnosticsErrorShowsStatus(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		return protocol.Message{}, errDiagBoom{}
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	m.files.document = &domain.FileDocument{Path: "x.py", Text: "x\n"}
	next, _ := m.Update(m.diagnosticsCommand("x.py")())
	m = next.(Model)
	if m.Status == "" {
		t.Fatal("error not reported")
	}
}

func TestOpenFileTriggersDiagnosticsFetch(t *testing.T) {
	var sent []protocol.Message
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		sent = append(sent, request)
		payload, _ := json.Marshal(protocol.LspDiagnosticsResult{Items: nil})
		return protocol.Message{Type: protocol.MessageLspDiagnosticsResult, WorkspaceID: "w", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelEditor
	next, cmd := m.Update(fileResult{
		kind:     protocol.MessageReadFile,
		response: protocol.FileResponse{Path: "x.py", Document: &domain.FileDocument{Path: "x.py", Text: "x = 1\n"}},
	})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("file open scheduled nothing")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	found := false
	for _, request := range sent {
		if request.Type != protocol.MessageLspDiagnostics {
			continue
		}
		var decoded protocol.LspDiagnosticsRequest
		if err := json.Unmarshal(request.Payload, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Path != "x.py" {
			t.Fatalf("request = %#v", decoded)
		}
		found = true
	}
	if !found {
		t.Fatalf("no diagnostics request sent: %#v", sent)
	}
	if m.Status != "no problems" {
		t.Fatalf("status = %q", m.Status)
	}
}

type errDiagBoom struct{}

func (errDiagBoom) Error() string { return "connection lost" }
