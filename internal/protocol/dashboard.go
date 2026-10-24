package protocol

// MessageTaskCancelRequested acknowledges a cancellation request, not process exit.
const MessageTaskCancelRequested MessageType = "tasks.cancel_requested"

const (
	MessageListTasks    MessageType = "tasks.list"
	MessageTaskPage     MessageType = "tasks.page"
	MessageTaskSnapshot MessageType = "tasks.snapshot"
	MessageTaskDetail   MessageType = "tasks.detail"
)

type TaskPageRequest struct {
	Before int64 `json:"before"`
}

// ApproveDecision carries a durable approval resolution from the client.
type ApproveDecision struct {
	ID       string `json:"id"`
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
}
