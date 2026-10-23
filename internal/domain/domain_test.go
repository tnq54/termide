package domain

import (
	"encoding/json"
	"testing"
)

func TestTaskTransitions(t *testing.T) {
	task := Task{ID: "task-1", Status: TaskQueued}
	if err := task.Transition(TaskRunning); err != nil {
		t.Fatalf("queued to running: %v", err)
	}
	if err := task.Transition(TaskCompleted); err != nil {
		t.Fatalf("running to completed: %v", err)
	}
	if err := task.Transition(TaskRunning); err == nil {
		t.Fatal("expected completed task to reject transition back to running")
	}
}

func TestAgentStatusesRoundTripJSON(t *testing.T) {
	agent := Agent{ID: "agent-1", TaskID: "task-1", Status: AgentWaitingApproval, Role: "tester"}
	data, err := json.Marshal(agent)
	if err != nil { t.Fatal(err) }
	var decoded Agent
	if err := json.Unmarshal(data, &decoded); err != nil { t.Fatal(err) }
	if decoded != agent { t.Fatalf("round trip mismatch: %#v != %#v", decoded, agent) }
}

func TestEventEnvelopeRequiresIdentity(t *testing.T) {
	if err := (EventEnvelope{Sequence: 1}).Validate(); err == nil {
		t.Fatal("expected missing event identity to fail validation")
	}
	if err := (EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 1, Type: EventAgentUpdated}).Validate(); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
}
