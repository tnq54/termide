package domain

type AgentStatus string

const (
	AgentQueued          AgentStatus = "queued"
	AgentPlanning        AgentStatus = "planning"
	AgentRunning         AgentStatus = "running"
	AgentWaitingApproval AgentStatus = "waiting_approval"
	AgentBlocked         AgentStatus = "blocked"
	AgentCompleted       AgentStatus = "completed"
	AgentFailed          AgentStatus = "failed"
	AgentCancelled       AgentStatus = "cancelled"
)

type Agent struct {
	ID         string      `json:"id"`
	TaskID     string      `json:"task_id"`
	Role       string      `json:"role"`
	Status     AgentStatus `json:"status"`
	Branch     string      `json:"branch,omitempty"`
	Worktree   string      `json:"worktree,omitempty"`
	Summary    string      `json:"summary,omitempty"`
}
