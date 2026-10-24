package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
)

func approveFixture() (Model, *[]protocol.Message) {
	var sent []protocol.Message
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		sent = append(sent, request)
		if request.Type == protocol.MessageApprove {
			return protocol.Message{
				Type:        protocol.MessageEvent,
				WorkspaceID: "w",
				TaskID:      "t",
				Event: &domain.EventEnvelope{
					WorkspaceID: "w", TaskID: "t", AgentID: "a",
					Sequence: 9, Type: domain.EventApproval,
				},
			}, nil
		}
		payload, _ := json.Marshal(domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskWaitingApproval}})
		return protocol.Message{Type: protocol.MessageTaskDetail, TaskID: "t", WorkspaceID: "w", Payload: payload}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{
		Task: domain.Task{ID: "t", WorkspaceID: "w", Description: "deploy", Status: domain.TaskWaitingApproval},
		Approvals: []domain.ApprovalRequest{
			{ID: "ap-1", TaskID: "t", AgentID: "a", Command: "rm -rf build", Reason: "cleanup", Status: domain.ApprovalPending},
		},
	}
	return m, &sent
}

func TestDashboardShowsPendingApproval(t *testing.T) {
	m, _ := approveFixture()
	view := m.View()
	if !strings.Contains(view, "rm -rf build") || !strings.Contains(view, "ap-1") {
		t.Fatal(view)
	}
}

func TestDashboardApproveSendsDecisionAndReloads(t *testing.T) {
	m, sent := approveFixture()
	next, cmd := m.Update(key("a"))
	m = next.(Model)
	if cmd == nil || !m.dashboard.busy {
		t.Fatal("approve not scheduled")
	}
	next, reload := m.Update(cmd())
	m = next.(Model)
	if reload == nil {
		t.Fatal("approval ack omitted reload")
	}
	if len(*sent) != 1 || (*sent)[0].Type != protocol.MessageApprove {
		t.Fatalf("sent = %#v", *sent)
	}
	var decision protocol.ApproveDecision
	if err := json.Unmarshal((*sent)[0].Payload, &decision); err != nil {
		t.Fatal(err)
	}
	if decision.ID != "ap-1" || !decision.Approved {
		t.Fatalf("decision = %#v", decision)
	}
	next, _ = m.Update(reload())
	m = next.(Model)
	if m.dashboard.busy {
		t.Fatal("reload did not finish")
	}
}

func TestDashboardDenySendsNegativeDecision(t *testing.T) {
	m, sent := approveFixture()
	next, cmd := m.Update(key("d"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("deny not scheduled")
	}
	next, _ = m.Update(cmd())
	_ = next
	if len(*sent) != 1 {
		t.Fatalf("sent = %#v", *sent)
	}
	var decision protocol.ApproveDecision
	if err := json.Unmarshal((*sent)[0].Payload, &decision); err != nil {
		t.Fatal(err)
	}
	if decision.ID != "ap-1" || decision.Approved {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestDashboardApproveRejectsForeignAck(t *testing.T) {
	remote := dashboardReply(func(request protocol.Message) (protocol.Message, error) {
		return protocol.Message{
			Type:        protocol.MessageEvent,
			WorkspaceID: "w",
			TaskID:      "other",
			Event:       &domain.EventEnvelope{WorkspaceID: "w", TaskID: "other", AgentID: "a", Sequence: 9, Type: domain.EventApproval},
		}, nil
	})
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{
		Task:      domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskWaitingApproval},
		Approvals: []domain.ApprovalRequest{{ID: "ap-1", TaskID: "t", AgentID: "a", Command: "rm -rf build", Status: domain.ApprovalPending}},
	}
	next, cmd := m.Update(key("a"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("approve not scheduled")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.dashboard.busy || m.dashboard.snapshot.Task.Status != domain.TaskWaitingApproval {
		t.Fatal("foreign ack mutated dashboard state")
	}
}
