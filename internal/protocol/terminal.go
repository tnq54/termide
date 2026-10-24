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
