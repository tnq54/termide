package domain

import "fmt"

type EventType string

const (
	EventTaskUpdated  EventType = "task.updated"
	EventAgentUpdated EventType = "agent.updated"
	EventOutput       EventType = "output"
	EventApproval     EventType = "approval"
)

type EventEnvelope struct {
	WorkspaceID string    `json:"workspace_id"`
	TaskID      string    `json:"task_id"`
	AgentID     string    `json:"agent_id"`
	Sequence    uint64    `json:"sequence"`
	Type        EventType `json:"type"`
	Payload     []byte    `json:"payload,omitempty"`
}

func (e EventEnvelope) Validate() error {
	if e.WorkspaceID == "" || e.TaskID == "" || e.AgentID == "" {
		return fmt.Errorf("event identity is required")
	}
	if e.Sequence == 0 {
		return fmt.Errorf("event sequence must be positive")
	}
	if e.Type == "" {
		return fmt.Errorf("event type is required")
	}
	return nil
}

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Root string `json:"root"`
}

type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalDenied   ApprovalStatus = "denied"
)

type ApprovalRequest struct {
	ID         string         `json:"id"`
	TaskID     string         `json:"task_id"`
	AgentID    string         `json:"agent_id"`
	Command    string         `json:"command"`
	Reason     string         `json:"reason"`
	WorkingDir string         `json:"working_dir"`
	Status     ApprovalStatus `json:"status"`
}

func (r ApprovalRequest) Validate() error {
	if r.ID == "" || r.TaskID == "" || r.AgentID == "" || r.Command == "" {
		return fmt.Errorf("approval identity and command are required")
	}
	return nil
}
