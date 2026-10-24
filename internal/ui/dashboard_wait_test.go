package ui

import (
	"context"
	"net"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/client"
	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/server"
	"termide/internal/store"
)

type waitRemote struct {
	waits  chan waitCall
	result dashboardWaitResult
	err    error
}

type waitCall struct {
	workspace string
	after     uint64
}

func (r *waitRemote) Call(context.Context, protocol.Message) (protocol.Message, error) {
	return protocol.Message{}, nil
}

func (r *waitRemote) Subscribe(_ context.Context, _ string, _ uint64) ([]domain.EventEnvelope, error) {
	return nil, nil
}

func (r *waitRemote) SubscribeWait(_ context.Context, workspace string, after uint64, _ time.Duration) ([]domain.EventEnvelope, error) {
	r.waits <- waitCall{workspace: workspace, after: after}
	return r.result.events, r.err
}

func waitFixture(result dashboardWaitResult, err error) (Model, *waitRemote) {
	remote := &waitRemote{waits: make(chan waitCall, 8), result: result, err: err}
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{
		Task:         domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning},
		NextSequence: 2,
	}
	return m, remote
}

func TestDashboardSnapshotSchedulesWaitForRunningTask(t *testing.T) {
	m, remote := waitFixture(dashboardWaitResult{}, nil)
	m.dashboard.waiting = false
	next, cmd := m.receiveDashboardWait(dashboardWaitResult{after: 2})
	m = next.(Model)
	if cmd == nil || !m.dashboard.waiting {
		t.Fatal("wait was not rescheduled")
	}
	go func() { _ = cmd() }()
	select {
	case call := <-remote.waits:
		if call.workspace != "w" || call.after != 2 {
			t.Fatalf("call = %#v", call)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait command did not call SubscribeWait")
	}
}

func TestDashboardSnapshotLoadSchedulesWait(t *testing.T) {
	m, remote := waitFixture(dashboardWaitResult{}, nil)
	m.dashboard.waiting = false
	m.dashboard.snapshot = nil
	next, cmd := m.receiveDashboard(dashboardResult{
		kind: protocol.MessageTaskSnapshot,
		snapshot: domain.TaskSnapshot{
			Task:         domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning},
			NextSequence: 0,
		},
	})
	m = next.(Model)
	if cmd == nil || !m.dashboard.waiting {
		t.Fatal("snapshot load did not schedule wait")
	}
	go func() { _ = cmd() }()
	select {
	case call := <-remote.waits:
		if call.after != 0 {
			t.Fatalf("call = %#v", call)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("scheduled wait did not run")
	}
}

func TestDashboardWaitAppendsNewEventsAndContinues(t *testing.T) {
	m, remote := waitFixture(dashboardWaitResult{}, nil)
	next, cmd := m.receiveDashboardWait(dashboardWaitResult{
		after: 2,
		events: []domain.EventEnvelope{
			{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 3, Type: domain.EventOutput, Payload: []byte(`{"Output":"live"}`)},
		},
	})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("wait did not continue after new events")
	}
	if m.dashboard.snapshot.NextSequence != 3 || len(m.dashboard.logs) != 1 {
		t.Fatalf("model = %#v", m.dashboard)
	}
	go func() { _ = cmd() }()
	select {
	case call := <-remote.waits:
		if call.after != 3 {
			t.Fatalf("resumed from stale cursor: %#v", call)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("continued wait did not run")
	}
}

func TestDashboardWaitErrorStopsLoop(t *testing.T) {
	m, _ := waitFixture(dashboardWaitResult{}, nil)
	next, cmd := m.receiveDashboardWait(dashboardWaitResult{after: 2, err: errWaitBoom()})
	m = next.(Model)
	if cmd != nil {
		t.Fatal("wait continued after error")
	}
	if m.dashboard.busy || m.Status == "" {
		t.Fatal("error was swallowed or dashboard locked")
	}
}

func TestDashboardWaitStopsOnTerminalTask(t *testing.T) {
	m, _ := waitFixture(dashboardWaitResult{}, nil)
	m.dashboard.snapshot.Task.Status = domain.TaskCompleted
	_, cmd := m.receiveDashboardWait(dashboardWaitResult{after: 2})
	if cmd != nil {
		t.Fatal("wait continued for terminal task")
	}
}

func TestDashboardWaitFiltersStaleEvents(t *testing.T) {
	m, _ := waitFixture(dashboardWaitResult{}, nil)
	m.dashboard.snapshot.NextSequence = 5
	next, _ := m.receiveDashboardWait(dashboardWaitResult{
		after: 2,
		events: []domain.EventEnvelope{
			{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 4, Type: domain.EventOutput, Payload: []byte(`{"Output":"old"}`)},
			{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 6, Type: domain.EventOutput, Payload: []byte(`{"Output":"new"}`)},
		},
	})
	m = next.(Model)
	if m.dashboard.snapshot.NextSequence != 6 || len(m.dashboard.logs) != 1 {
		t.Fatalf("model = %#v", m.dashboard)
	}
}

func TestDashboardWaitEndToEndOverTransport(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w"}); err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	done := make(chan error, 1)
	go func() {
		defer right.Close()
		done <- server.NewHandler(s).Serve(ctx, right, right)
	}()
	remote := client.NewRemote(left)
	defer func() { remote.Close(); <-done }()
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{
		Task:         domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning},
		NextSequence: 0,
	}
	cmd := m.waitCommand()
	if cmd == nil {
		t.Fatal("real remote did not schedule wait")
	}
	msgDone := make(chan tea.Msg, 1)
	go func() { msgDone <- cmd() }()
	time.AfterFunc(300*time.Millisecond, func() {
		_ = s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Type: domain.EventOutput, Payload: []byte(`{"Output":"live"}`)})
	})
	select {
	case msg := <-msgDone:
		result, ok := msg.(dashboardWaitResult)
		if !ok || result.err != nil || len(result.events) != 1 {
			t.Fatalf("msg = %#v", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("transport wait did not wake on new event")
	}
}

func TestDashboardKeysWorkWhileWaiting(t *testing.T) {
	m, _ := waitFixture(dashboardWaitResult{}, nil)
	m.dashboard.waiting = true
	for i := 0; i < 40; i++ {
		m.dashboard.logs = append(m.dashboard.logs, "line")
	}
	next, cmd := m.Update(key("End"))
	m = next.(Model)
	if cmd != nil || m.dashboard.busy {
		t.Fatal("scroll blocked while waiting")
	}
}

type waitError string

func (e waitError) Error() string { return string(e) }

func errWaitBoom() error { return waitError("wait failed") }
