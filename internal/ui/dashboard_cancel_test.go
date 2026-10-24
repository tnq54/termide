package ui

import (
	"encoding/json"
	"fmt"
	"termide/internal/domain"
	"termide/internal/protocol"
	"testing"
)

func TestDashboardCancelRequestsAndReloadsWithoutInventingStatus(t *testing.T) {
	calls := 0
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		calls++
		if request.TaskID != "t" || request.WorkspaceID != "w" {
			t.Fatal(request)
		}
		if calls == 1 {
			if request.Type != protocol.MessageCancel {
				t.Fatal(request.Type)
			}
			return protocol.Message{Type: protocol.MessageTaskCancelRequested, TaskID: "t", WorkspaceID: "w"}, nil
		}
		if request.Type != protocol.MessageTaskSnapshot {
			t.Fatal(request.Type)
		}
		payload, _ := json.Marshal(domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning}})
		return protocol.Message{Type: protocol.MessageTaskDetail, TaskID: "t", WorkspaceID: "w", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning}}
	next, cmd := m.Update(key("c"))
	m = next.(Model)
	if cmd == nil || !m.dashboard.busy {
		t.Fatal("cancel not scheduled")
	}
	if _, duplicate := m.Update(key("c")); duplicate != nil {
		t.Fatal("duplicate cancellation in flight")
	}
	next, reload := m.Update(cmd())
	m = next.(Model)
	if reload == nil || m.dashboard.snapshot.Task.Status != domain.TaskRunning {
		t.Fatal("cancel ack invented terminal status or omitted reload")
	}
	next, _ = m.Update(reload())
	m = next.(Model)
	if m.dashboard.busy || m.dashboard.snapshot.Task.Status != domain.TaskRunning || calls != 2 {
		t.Fatal("reload failed")
	}
}

func TestDashboardCancelFailureRetainsTask(t *testing.T) {
	m := NewRemoteModel(dashboardReply(func(protocol.Message) (protocol.Message, error) {
		return protocol.Message{}, fmt.Errorf("connection lost")
	}), domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{ID: "t", Status: domain.TaskRunning}}
	next, cmd := m.Update(key("c"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("cancel not scheduled")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.dashboard.busy || m.dashboard.snapshot.Task.Status != domain.TaskRunning || m.Status != "connection lost" {
		t.Fatal(m.Status)
	}
}

func TestDashboardCancelUnavailableOfflineOrTerminal(t *testing.T) {
	for _, ready := range []bool{true, false} {
		m := NewRemoteModel(dashboardReply(func(protocol.Message) (protocol.Message, error) {
			t.Fatal("unexpected IO")
			return protocol.Message{}, nil
		}), domain.Workspace{ID: "w"})
		m.ready = ready
		m.Panel = PanelAgents
		status := domain.TaskCompleted
		if !ready {
			status = domain.TaskRunning
		}
		m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{ID: "t", Status: status}}
		next, cmd := m.Update(key("c"))
		if cmd != nil || next.(Model).dashboard.busy {
			t.Fatal("invalid cancel scheduled")
		}
	}
}
