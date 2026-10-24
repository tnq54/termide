package ui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
	"termide/internal/protocol"
)

func terminalFixture() (Model, *protocol.Message) {
	var sent protocol.Message
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		sent = request
		payload, _ := json.Marshal(protocol.CommandResult{Output: "hello\n", ExitCode: 0})
		return protocol.Message{Type: protocol.MessageCommandResult, WorkspaceID: "w", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelTerminal
	return m, &sent
}

func TestTerminalRunsTypedCommand(t *testing.T) {
	m, sent := terminalFixture()
	next, _ := m.Update(key("pwd"))
	m = next.(Model)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil || !m.termBusy {
		t.Fatal("command not scheduled")
	}
	run := cmd
	next, busyCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if busyCmd != nil {
		t.Fatal("second command scheduled while busy")
	}
	_ = next
	next, _ = m.Update(run())
	m = next.(Model)
	if m.termBusy {
		t.Fatal("busy not cleared")
	}
	if sent.Type != protocol.MessageRunCommand {
		t.Fatalf("sent = %#v", sent)
	}
	var request protocol.CommandRequest
	if err := json.Unmarshal(sent.Payload, &request); err != nil || request.Command != "pwd" {
		t.Fatalf("request = %#v", request)
	}
	if !strings.Contains(m.View(), "hello") || !strings.Contains(m.View(), "pwd") {
		t.Fatalf("view = %q", m.View())
	}
}

func TestTerminalErrorRetainsInput(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		return protocol.Message{}, errTerminalBoom{}
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelTerminal
	next, _ := m.Update(key("pwd"))
	m = next.(Model)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("command not scheduled")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.termBusy || m.termInput != "pwd" || m.Status == "" {
		t.Fatalf("model = %#v %q", m, m.Status)
	}
}

func TestTerminalRejectsForeignResponse(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		payload, _ := json.Marshal(protocol.CommandResult{Output: "x"})
		return protocol.Message{Type: protocol.MessageCommandResult, WorkspaceID: "other", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelTerminal
	next, _ := m.Update(key("pwd"))
	m = next.(Model)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.termBusy || len(m.termOutput) != 0 {
		t.Fatal("foreign response accepted")
	}
}

func TestTerminalScrollStaysInBounds(t *testing.T) {
	m, _ := terminalFixture()
	for i := 0; i < 50; i++ {
		m.termOutput = append(m.termOutput, "line")
	}
	for i := 0; i < 100; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(Model)
	}
	if m.termScroll < 0 || m.termScroll > len(m.termOutput) {
		t.Fatalf("scroll = %d", m.termScroll)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if next.(Model).Panel != PanelDashboard {
		t.Fatal("escape did not leave terminal")
	}
}

type errTerminalBoom struct{}

func (errTerminalBoom) Error() string { return "connection lost" }
