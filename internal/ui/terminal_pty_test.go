package ui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
	"termide/internal/protocol"
)

type ptyFake struct {
	opened   bool
	written  []byte
	reads    int
	failOpen bool
}

type errPtyBoom struct{}

func (errPtyBoom) Error() string { return "connection lost" }

func (f *ptyFake) remote() dashboardReply {
	return dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		payload := func(value any) []byte {
			raw, _ := json.Marshal(value)
			return raw
		}
		switch request.Type {
		case protocol.MessagePtyOpen:
			if f.failOpen {
				return protocol.Message{}, errPtyBoom{}
			}
			f.opened = true
			return protocol.Message{Type: protocol.MessagePtyOpened, WorkspaceID: "w", Payload: payload(protocol.PtyOpened{ID: "pty-1"})}, nil
		case protocol.MessagePtyWrite:
			var target protocol.PtyTarget
			_ = json.Unmarshal(request.Payload, &target)
			f.written = append(f.written, target.Data...)
			return protocol.Message{Type: protocol.MessagePtyOk, WorkspaceID: "w", Payload: payload(struct{}{})}, nil
		case protocol.MessagePtyRead:
			f.reads++
			out := ""
			if f.reads == 1 {
				out = "hello-pty\r\n$ "
			}
			return protocol.Message{Type: protocol.MessagePtyOutput, WorkspaceID: "w", Payload: payload(protocol.PtyOutput{ID: "pty-1", Data: []byte(out), Next: uint64(len(out))})}, nil
		case protocol.MessagePtyResize, protocol.MessagePtyClose:
			return protocol.Message{Type: protocol.MessagePtyOk, WorkspaceID: "w", Payload: payload(struct{}{})}, nil
		}
		return protocol.Message{}, errPtyBoom{}
	})
}

func ptyFixture(fake *ptyFake) Model {
	m := NewRemoteModel(fake.remote(), domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelTerminal
	m.Width, m.Height = 60, 20
	return m
}

func TestTerminalOpensPtyOnEntry(t *testing.T) {
	fake := &ptyFake{}
	m := NewRemoteModel(fake.remote(), domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelDashboard
	m.Width, m.Height = 60, 20
	next, cmd := m.Update(key("t"))
	m = next.(Model)
	if m.Panel != PanelTerminal || cmd == nil {
		t.Fatal("terminal entry did not schedule open")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.termPtyID != "pty-1" || !m.termPtyOpen {
		t.Fatalf("session not opened: %#v", m)
	}
}

func TestTerminalKeystrokeWritesToPty(t *testing.T) {
	fake := &ptyFake{}
	m := ptyFixture(fake)
	m.termPtyID = "pty-1"
	m.termPtyOpen = true
	next, cmd := m.Update(key("x"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("keystroke not forwarded")
	}
	next, _ = m.Update(cmd())
	_ = next
	if string(fake.written) != "x" {
		t.Fatalf("written = %q", fake.written)
	}
}

func TestTerminalPollAppendsOutput(t *testing.T) {
	fake := &ptyFake{}
	m := ptyFixture(fake)
	m.termPtyID = "pty-1"
	m.termPtyOpen = true
	msg := m.termReadCommand()()
	next, follow := m.Update(msg)
	m = next.(Model)
	if follow == nil {
		t.Fatal("poll did not continue")
	}
	if !strings.Contains(strings.Join(m.termOutput, "\n"), "hello-pty") {
		t.Fatalf("output = %q", m.termOutput)
	}
}

func TestTerminalScrollStaysInBounds(t *testing.T) {
	fake := &ptyFake{}
	m := ptyFixture(fake)
	m.termPtyID = "pty-1"
	m.termPtyOpen = true
	for i := 0; i < 50; i++ {
		m.termOutput = append(m.termOutput, "line")
	}
	for i := 0; i < 100; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = next.(Model)
	}
	if m.termScroll < 0 || m.termScroll > len(m.termOutput) {
		t.Fatalf("scroll = %d", m.termScroll)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = next.(Model)
	if m.termScroll >= len(m.termOutput) {
		t.Fatalf("scroll = %d", m.termScroll)
	}
}

func TestTerminalEscapeClosesSession(t *testing.T) {
	fake := &ptyFake{}
	m := ptyFixture(fake)
	m.termPtyID = "pty-1"
	m.termPtyOpen = true
	var closed []protocol.Message
	remote := m.remote.(dashboardReply)
	m.remote = dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		closed = append(closed, request)
		return remote(request)
	})
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if cmd != nil {
		_ = cmd()
	}
	if m.Panel != PanelDashboard || m.termPtyID != "" {
		t.Fatal("escape did not close session")
	}
	found := false
	for _, message := range closed {
		if message.Type == protocol.MessagePtyClose {
			found = true
		}
	}
	if !found {
		t.Fatal("pty.close was not sent")
	}
}

func TestTerminalOpenErrorStaysUsable(t *testing.T) {
	fake := &ptyFake{failOpen: true}
	m := NewRemoteModel(fake.remote(), domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelDashboard
	m.Width, m.Height = 60, 20
	next, cmd := m.Update(key("t"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("open not scheduled")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.termPtyID != "" || m.Status == "" {
		t.Fatal("open failure not reported")
	}
	next, keyCmd := m.Update(key("x"))
	m = next.(Model)
	if keyCmd != nil {
		t.Fatal("keystroke sent without session")
	}
}
