package protocol

// MessageTaskCancelRequested acknowledges a cancellation request, not process exit.
const MessageTaskCancelRequested MessageType = "tasks.cancel_requested"

const (
	MessageListTasks    MessageType = "tasks.list"
	MessageTaskPage     MessageType = "tasks.page"
	MessageTaskSnapshot MessageType = "tasks.snapshot"
	MessageTaskDetail   MessageType = "tasks.detail"
	MessageTaskDiff     MessageType = "tasks.diff"
	MessageTaskDiffs    MessageType = "tasks.diffs"
)

type TaskPageRequest struct {
	Before int64 `json:"before"`
}

// TaskDiff carries one agent's worktree changes for review. Patch is capped
// server-side; Truncated marks a cut patch and Error carries per-agent git
// failures without failing the whole review.
type TaskDiff struct {
	AgentID   string   `json:"agent_id"`
	Branch    string   `json:"branch,omitempty"`
	Worktree  string   `json:"worktree,omitempty"`
	Files     []string `json:"files"`
	Patch     string   `json:"patch,omitempty"`
	Truncated bool     `json:"truncated,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// ApproveDecision carries a durable approval resolution from the client.
type ApproveDecision struct {
	ID       string `json:"id"`
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
}
