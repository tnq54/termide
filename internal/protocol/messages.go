package protocol

import (
	"fmt"
	"termide/internal/domain"
)

type MessageType string

const MessageOpenWorkspace MessageType = "open_workspace"
const MessageWorkspaceReady MessageType = "workspace_ready"

const (
	MessageHello      MessageType = "hello"
	MessageSubscribe  MessageType = "subscribe"
	MessageCreateTask MessageType = "create_task"
	MessageStartTask  MessageType = "start_task"
	MessageApprove    MessageType = "approve"
	MessageCancel     MessageType = "cancel"
	MessageRunCommand MessageType = "run_command"
	MessageEvent      MessageType = "event"
	MessageError      MessageType = "error"
)

type Message struct {
	Type          MessageType           `json:"type"`
	RequestID     string                `json:"request_id,omitempty"`
	WorkspaceID   string                `json:"workspace_id,omitempty"`
	TaskID        string                `json:"task_id,omitempty"`
	AfterSequence uint64                `json:"after_sequence,omitempty"`
	Payload       []byte                `json:"payload,omitempty"`
	Event         *domain.EventEnvelope `json:"event,omitempty"`
}

func (m Message) Validate() error {
	if m.Type == "" {
		return fmt.Errorf("message type is required")
	}
	if m.Type == MessageEvent {
		if m.Event == nil {
			return fmt.Errorf("event payload is required")
		}
		return m.Event.Validate()
	}
	return nil
}
