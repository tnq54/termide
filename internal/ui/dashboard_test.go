package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"termide/internal/agent"
	"termide/internal/client"
	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/server"
	"termide/internal/store"
)

type dashboardRunner struct{}

func (dashboardRunner) Run(_ context.Context, spec agent.Spec, emit func(agent.Event)) error {
	emit(agent.Event{AgentID: spec.ID, Output: "completed output"})
	return nil
}
func TestDashboardStartThroughTransportReloadsFinalStatus(t *testing.T) {
	m, s := dashboardFixture(t)
	if err := s.UpdateTaskStatus(context.Background(), "task-a", domain.TaskQueued); err != nil {
		t.Fatal(err)
	}
	m = applyCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	next, start := m.Update(key("s"))
	m = next.(Model)
	if start == nil {
		t.Fatal("start not scheduled")
	}
	next, reload := m.Update(start())
	m = next.(Model)
	if reload == nil {
		t.Fatalf("final snapshot not requested: %s", m.Status)
	}
	next, _ = m.Update(reload())
	m = next.(Model)
	deadline := time.Now().Add(time.Second)
	for m.dashboard.snapshot.Task.Status == domain.TaskRunning && time.Now().Before(deadline) {
		m = applyCommand(t, m, key("r"))
	}
	if m.dashboard.snapshot.Task.Status != domain.TaskCompleted || !strings.Contains(strings.Join(m.dashboard.logs, "\n"), "completed output") {
		t.Fatal(m.dashboard.snapshot)
	}
}

type startCapture struct{ request protocol.Message }

func (r *startCapture) Call(_ context.Context, request protocol.Message) (protocol.Message, error) {
	r.request = request
	return protocol.Message{Type: protocol.MessageEvent, WorkspaceID: request.WorkspaceID, TaskID: request.TaskID, Event: &domain.EventEnvelope{WorkspaceID: request.WorkspaceID, TaskID: request.TaskID, AgentID: "system", Sequence: 1, Type: domain.EventTaskUpdated}}, nil
}
func TestDashboardStartUsesDescriptionAndUniqueAgent(t *testing.T) {
	remote := &startCapture{}
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{ID: "t", Description: "sửa lỗi Python", Status: domain.TaskQueued}}
	var previous string
	for i := 0; i < 2; i++ {
		cmd := m.dashboardCommand(protocol.MessageStartTask, "t", 0, 0)
		if result := cmd().(dashboardResult); result.err != nil {
			t.Fatal(result.err)
		}
		var specs []map[string]string
		if err := json.Unmarshal(remote.request.Payload, &specs); err != nil {
			t.Fatal(err)
		}
		if len(specs) != 1 || specs[0]["command"] != "sửa lỗi Python" {
			t.Fatal(specs)
		}
		if specs[0]["id"] == "" || specs[0]["id"] == previous {
			t.Fatal("agent ID reused")
		}
		previous = specs[0]["id"]
	}
}

func TestDashboardOfflineRefreshDoesNotScheduleIO(t *testing.T) {
	m := NewModel()
	next, cmd := m.Update(key("r"))
	if cmd != nil || next.(Model).dashboard.busy {
		t.Fatal("offline refresh scheduled IO or locked dashboard")
	}
}

func TestDashboardScrollKeepsSelectionAndStatusVisible(t *testing.T) {
	m := NewModel()
	m.Width = 24
	m.Height = 8
	m.Status = "connected"
	for i := 0; i < 30; i++ {
		m.dashboard.tasks = append(m.dashboard.tasks, domain.Task{Description: fmt.Sprintf("task-%02d", i)})
	}
	for i := 0; i < 29; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(Model)
	}
	if !strings.Contains(m.View(), "> task-29") || !strings.Contains(m.View(), "connected") {
		t.Fatal(m.View())
	}
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{Description: "details"}}
	for i := 0; i < 30; i++ {
		m.dashboard.logs = append(m.dashboard.logs, fmt.Sprintf("log-%02d", i))
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = next.(Model)
	if !strings.Contains(m.View(), "log-29") || !strings.Contains(m.View(), "connected") {
		t.Fatal(m.View())
	}
}

func TestDashboardWideUnicodeFitsCells(t *testing.T) {
	m := NewModel()
	m.Width = 9
	m.Height = 8
	m.dashboard.tasks = []domain.Task{{Description: strings.Repeat("界", 20)}}
	for _, line := range strings.Split(m.View(), "\n") {
		if ansi.StringWidth(line) > m.Width {
			t.Fatal(line)
		}
	}
}

func TestDashboardEmptyViewportEverySmallHeight(t *testing.T) {
	for h := 1; h <= 12; h++ {
		m := NewModel()
		m.Width = 20
		m.Height = h
		if len(strings.Split(m.View(), "\n")) > h {
			t.Fatalf("height %d: %q", h, m.View())
		}
	}
}

func dashboardFixture(t *testing.T) (Model, *store.Store) {
	t.Helper()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	workspace := domain.Workspace{ID: "w", Root: t.TempDir()}
	s.CreateWorkspace(context.Background(), workspace)
	s.CreateTask(context.Background(), domain.Task{ID: "task-a", WorkspaceID: "w", Description: "git status", Status: domain.TaskRunning})
	s.UpsertAgent(context.Background(), domain.Agent{ID: "agent-a", TaskID: "task-a", Role: "coder", Status: domain.AgentRunning})
	payload, _ := json.Marshal(map[string]string{"Output": "hello từ VPS"})
	s.AppendEvent(context.Background(), domain.EventEnvelope{WorkspaceID: "w", TaskID: "task-a", AgentID: "agent-a", Type: domain.EventOutput, Payload: payload})
	left, right := net.Pipe()
	done := make(chan error, 1)
	go func() {
		defer right.Close()
		done <- server.NewHandlerWithRunner(s, dashboardRunner{}).Serve(context.Background(), right, right)
	}()
	remote := client.NewRemote(left)
	t.Cleanup(func() { remote.Close(); <-done; s.Close() })
	m := NewRemoteModel(remote, workspace)
	next, load := m.Update(m.Init()())
	m = next.(Model)
	if load == nil {
		t.Fatal("opening workspace did not load task history")
	}
	next, _ = m.Update(load())
	m = next.(Model)
	next, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 18})
	m = next.(Model)
	return m, s
}

func TestDashboardRestoresTaskHistoryAndRefreshesIncrementalLogs(t *testing.T) {
	m, s := dashboardFixture(t)
	if len(m.dashboard.tasks) != 1 || !strings.Contains(m.View(), "git status") {
		t.Fatal(m.View())
	}
	m = applyCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Panel != PanelAgents || m.dashboard.snapshot == nil || !strings.Contains(m.View(), "hello từ VPS") {
		t.Fatal(m.View())
	}
	payload, _ := json.Marshal(map[string]string{"Output": "finished"})
	s.AppendEvent(context.Background(), domain.EventEnvelope{WorkspaceID: "w", TaskID: "task-a", AgentID: "agent-a", Type: domain.EventOutput, Payload: payload})
	s.UpdateTaskStatus(context.Background(), "task-a", domain.TaskCompleted)
	m = applyCommand(t, m, key("r"))
	if m.dashboard.snapshot.Task.Status != domain.TaskCompleted {
		t.Fatal("status not refreshed")
	}
	if len(m.dashboard.logs) != 2 {
		t.Fatal("missing or duplicate log lines")
	}
	m = applyCommand(t, m, key("r"))
	if len(m.dashboard.logs) != 2 {
		t.Fatal("logs duplicated")
	}
}

func TestDashboardSanitizesAgentIDInLogs(t *testing.T) {
	m, s := dashboardFixture(t)
	s.AppendEvent(context.Background(), domain.EventEnvelope{WorkspaceID: "w", TaskID: "task-a", AgentID: "bad\x1b[2J", Type: domain.EventOutput, Payload: []byte(`{"Output":"safe"}`)})
	m = applyCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if strings.Contains(m.View(), "\x1b") {
		t.Fatal("agent ID control sequence leaked")
	}
}

func TestDashboardSmallViewportAndUntrustedOutput(t *testing.T) {
	m, s := dashboardFixture(t)
	payload, _ := json.Marshal(map[string]string{"Output": "\x1b[2Jmalicious\rcontent"})
	s.AppendEvent(context.Background(), domain.EventEnvelope{WorkspaceID: "w", TaskID: "task-a", AgentID: "agent-a", Type: domain.EventOutput, Payload: payload})
	m = applyCommand(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	for _, size := range [][2]int{{40, 12}, {20, 8}, {1, 1}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(Model)
		view := m.View()
		if strings.Contains(view, "\x1b[2J") {
			t.Fatal("terminal control sequence leaked")
		}
		lines := strings.Split(view, "\n")
		if len(lines) > size[1] {
			t.Fatal("height overflow")
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("width overflow")
			}
		}
	}
}
