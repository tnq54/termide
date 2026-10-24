package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"termide/internal/domain"
	"termide/internal/protocol"
	"testing"
)

type dashboardReply func(protocol.Message) (protocol.Message, error)

func TestDashboardRejectsMismatchedStartEvent(t *testing.T) {
	for _, missing := range []bool{true, false} {
		remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
			r := protocol.Message{Type: protocol.MessageEvent, TaskID: "t", WorkspaceID: "w"}
			if !missing {
				r.Event = &domain.EventEnvelope{TaskID: "other", WorkspaceID: "w", AgentID: "system", Sequence: 1, Type: domain.EventTaskUpdated}
			}
			return r, nil
		})
		m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
		m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{ID: "t"}}
		if r := m.dashboardCommand(protocol.MessageStartTask, "t", 0, 0)().(dashboardResult); r.err == nil {
			t.Fatal("accepted missing/foreign start event")
		}
	}
}

func (f dashboardReply) Call(_ context.Context, m protocol.Message) (protocol.Message, error) {
	return f(m)
}

func TestDashboardRejectsMismatchedSnapshotAndSequence(t *testing.T) {
	for _, mutation := range []string{"outer workspace", "task", "workspace", "agent", "event task", "event workspace", "replayed sequence", "cursor", "empty more"} {
		t.Run(mutation, func(t *testing.T) {
			snapshot := domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w"}, Agents: []domain.Agent{{ID: "a", TaskID: "t"}}, Events: []domain.EventEnvelope{{TaskID: "t", WorkspaceID: "w", AgentID: "a", Sequence: 6, Type: domain.EventOutput}}, NextSequence: 6}
			outer := "w"
			switch mutation {
			case "outer workspace":
				outer = "other"
			case "task":
				snapshot.Task.ID = "other"
			case "workspace":
				snapshot.Task.WorkspaceID = "other"
			case "agent":
				snapshot.Agents[0].TaskID = "other"
			case "event task":
				snapshot.Events[0].TaskID = "other"
			case "event workspace":
				snapshot.Events[0].WorkspaceID = "other"
			case "replayed sequence":
				snapshot.Events[0].Sequence = 5
			case "cursor":
				snapshot.NextSequence = 100
			case "empty more":
				snapshot.Events = nil
				snapshot.NextSequence = 5
				snapshot.More = true
			}
			payload, _ := json.Marshal(snapshot)
			remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
				return protocol.Message{Type: protocol.MessageTaskDetail, WorkspaceID: outer, TaskID: "t", Payload: payload}, nil
			})
			m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
			result := m.dashboardCommand(protocol.MessageTaskSnapshot, "t", 0, 5)().(dashboardResult)
			if result.err == nil {
				t.Fatal("accepted invalid snapshot")
			}
		})
	}
}

func TestDashboardRejectsForeignPageTask(t *testing.T) {
	payload, _ := json.Marshal(domain.TaskPage{Tasks: []domain.Task{{ID: "t", WorkspaceID: "other"}}})
	remote := dashboardReply(func(protocol.Message) (protocol.Message, error) {
		return protocol.Message{Type: protocol.MessageTaskPage, WorkspaceID: "w", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	if result := m.dashboardCommand(protocol.MessageListTasks, "", 0, 0)().(dashboardResult); result.err == nil {
		t.Fatal("accepted foreign task")
	}
}

func TestDashboardRefreshFailurePreservesPagination(t *testing.T) {
	m := NewRemoteModel(dashboardReply(func(protocol.Message) (protocol.Message, error) {
		return protocol.Message{}, fmt.Errorf("temporary failure")
	}), domain.Workspace{ID: "w"})
	m.ready = true
	m.dashboard.tasks = []domain.Task{{ID: "t", WorkspaceID: "w"}}
	m.dashboard.before = 42
	next, cmd := m.Update(key("r"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.dashboard.before != 42 || len(m.dashboard.tasks) != 1 {
		t.Fatal("failed refresh lost pagination")
	}
}

func TestDashboardPaginationAndRefreshThroughServer(t *testing.T) {
	m, s := dashboardFixture(t)
	for i := 0; i < 55; i++ {
		if err := s.CreateTask(context.Background(), domain.Task{ID: fmt.Sprintf("page-%d", i), WorkspaceID: "w", Status: domain.TaskQueued}); err != nil {
			t.Fatal(err)
		}
	}
	m = applyCommand(t, m, key("r"))
	if len(m.dashboard.tasks) != 50 || m.dashboard.before == 0 {
		t.Fatal("missing first page")
	}
	m = applyCommand(t, m, key("l"))
	if len(m.dashboard.tasks) != 56 || m.dashboard.before != 0 {
		t.Fatal("missing last page")
	}
	m = applyCommand(t, m, key("r"))
	if len(m.dashboard.tasks) != 50 {
		t.Fatal("refresh appended rather than replaced")
	}
	seen := map[string]bool{}
	for _, task := range m.dashboard.tasks {
		if seen[task.ID] {
			t.Fatal("duplicate task")
		}
		seen[task.ID] = true
	}
}
