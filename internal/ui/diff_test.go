package ui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
	"termide/internal/protocol"
)

func diffFixture() (Model, *[]protocol.Message) {
	var sent []protocol.Message
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		sent = append(sent, request)
		payload, _ := json.Marshal([]protocol.TaskDiff{
			{AgentID: "a", Branch: "termide/t/a", Files: []string{"main.py"}, Patch: "+print('review me')\n"},
		})
		return protocol.Message{Type: protocol.MessageTaskDiffs, WorkspaceID: "w", TaskID: "t", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskCompleted}}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m = next.(Model)
	return m, &sent
}

func TestDiffOpensFromDetailAndRendersPatch(t *testing.T) {
	m, sent := diffFixture()
	next, cmd := m.Update(key("f"))
	m = next.(Model)
	if cmd == nil || !m.diff.busy {
		t.Fatal("diff not scheduled")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.Panel != PanelDiff || m.diff.taskID != "t" {
		t.Fatalf("panel = %q task = %q", m.Panel, m.diff.taskID)
	}
	if len(*sent) != 1 || (*sent)[0].Type != protocol.MessageTaskDiff {
		t.Fatalf("sent = %#v", sent)
	}
	view := m.View()
	if !strings.Contains(view, "review me") || !strings.Contains(view, "main.py") {
		t.Fatalf("view = %q", view)
	}
}

func TestDiffOpensFromTaskList(t *testing.T) {
	var sent []protocol.Message
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		sent = append(sent, request)
		payload, _ := json.Marshal([]protocol.TaskDiff{})
		return protocol.Message{Type: protocol.MessageTaskDiffs, WorkspaceID: "w", TaskID: "task-a", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelDashboard
	m.dashboard.tasks = []domain.Task{{ID: "task-a", WorkspaceID: "w", Description: "job"}}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m = next.(Model)
	next, cmd := m.Update(key("f"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("diff not scheduled from list")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.Panel != PanelDiff || len(sent) != 1 || sent[0].TaskID != "task-a" {
		t.Fatalf("panel = %q sent = %#v", m.Panel, sent)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if next.(Model).Panel != PanelDashboard {
		t.Fatal("escape did not return to dashboard")
	}
}

func TestDiffScrollStaysInBounds(t *testing.T) {
	m, _ := diffFixture()
	next, cmd := m.Update(key("f"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	for i := 0; i < 100; i++ {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(Model)
	}
	if m.diff.scroll < 0 || m.diff.scroll > len(m.diff.lines) {
		t.Fatalf("scroll = %d", m.diff.scroll)
	}
}

func TestDiffEscapeReturnsToDetail(t *testing.T) {
	m, _ := diffFixture()
	next, cmd := m.Update(key("f"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.Panel != PanelAgents || m.diff.taskID != "" {
		t.Fatalf("panel = %q", m.Panel)
	}
}

func TestDiffErrorRetainsDetail(t *testing.T) {
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		return protocol.Message{}, errDiffBoom{}
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskCompleted}}
	next, cmd := m.Update(key("f"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("diff not scheduled")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.Panel != PanelAgents || m.diff.busy || m.Status == "" {
		t.Fatal("error did not retain detail")
	}
}

func TestDiffUnavailableOffline(t *testing.T) {
	m := NewModel()
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{ID: "t", Status: domain.TaskCompleted}}
	next, cmd := m.Update(key("f"))
	if cmd != nil || next.(Model).diff.busy {
		t.Fatal("offline diff scheduled IO")
	}
}

type errDiffBoom struct{}

func (errDiffBoom) Error() string { return "connection lost" }
