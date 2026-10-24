package ui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
	"termide/internal/protocol"
)

type replayRemote struct {
	events []domain.EventEnvelope
	calls  int
}

func (r *replayRemote) Call(context.Context, protocol.Message) (protocol.Message, error) {
	return protocol.Message{}, nil
}

func (r *replayRemote) Subscribe(_ context.Context, workspaceID string, after uint64) ([]domain.EventEnvelope, error) {
	r.calls++
	if workspaceID != "w" || after != 2 {
		return nil, testingError("unexpected replay cursor")
	}
	return r.events, nil
}

type testingError string

func (e testingError) Error() string { return string(e) }

func TestDashboardReplaysEventsAfterReconnectCursor(t *testing.T) {
	remote := &replayRemote{events: []domain.EventEnvelope{{
		WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 3,
		Type: domain.EventOutput, Payload: []byte(`{"Output":"resumed"}`),
	}}}
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{
		Task:         domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning},
		NextSequence: 2,
		Events:       []domain.EventEnvelope{{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 2, Type: domain.EventOutput}},
	}

	cmd := m.dashboardReplayCommand()
	result := cmd().(dashboardReplayResult)
	if result.err != nil {
		t.Fatal(result.err)
	}
	next, _ := m.receiveDashboardReplay(result)
	m = next.(Model)
	if remote.calls != 1 || len(m.dashboard.logs) != 1 || m.dashboard.logs[0] != "[a] resumed" {
		t.Fatalf("model after replay: %#v", m.dashboard)
	}
	if m.dashboard.snapshot.NextSequence != 3 {
		t.Fatalf("next sequence = %d", m.dashboard.snapshot.NextSequence)
	}
}

var _ tea.Msg = dashboardReplayResult{}
