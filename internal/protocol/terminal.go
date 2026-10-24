package protocol

// CommandRequest asks the server to run one workspace-scoped shell command.
type CommandRequest struct {
	Command string `json:"command"`
}

// CommandResult carries bounded combined output of a finished command.
type CommandResult struct {
	Output    string `json:"output"`
	ExitCode  int    `json:"exit_code"`
	Truncated bool   `json:"truncated,omitempty"`
}

const (
	MessagePtyOpen   MessageType = "pty.open"
	MessagePtyWrite  MessageType = "pty.write"
	MessagePtyRead   MessageType = "pty.read"
	MessagePtyResize MessageType = "pty.resize"
	MessagePtyClose  MessageType = "pty.close"

	MessagePtyOpened MessageType = "pty.opened"
	MessagePtyOutput MessageType = "pty.output"
	MessagePtyOk     MessageType = "pty.ok"
)

// PtyOpenRequest asks the server to start an interactive shell in the
// workspace root.
type PtyOpenRequest struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

// PtyTarget addresses one session for write/read/resize/close.
type PtyTarget struct {
	ID   string `json:"id"`
	Data []byte `json:"data,omitempty"`
	// Read uses After; resize uses Rows/Cols.
	After uint64 `json:"after,omitempty"`
	Rows  int    `json:"rows,omitempty"`
	Cols  int    `json:"cols,omitempty"`
}

// PtyOpened acknowledges a new session.
type PtyOpened struct {
	ID string `json:"id"`
}

// PtyOutput carries output after an absolute offset plus the new end offset.
type PtyOutput struct {
	ID   string `json:"id"`
	Data []byte `json:"data"`
	Next uint64 `json:"next"`
}
