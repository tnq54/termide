package protocol

const (
	MessageLspHover             MessageType = "lsp.hover"
	MessageLspHoverResult       MessageType = "lsp.hover_result"
	MessageLspDiagnostics       MessageType = "lsp.diagnostics"
	MessageLspDiagnosticsResult MessageType = "lsp.diagnostics_result"
)

// LspHoverRequest asks for hover contents at a zero-based line and rune-offset
// character in a workspace-relative file.
type LspHoverRequest struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
}

// LspHoverResult carries rendered hover contents as plain text.
type LspHoverResult struct {
	Contents string `json:"contents"`
}

// LspDiagnosticsRequest asks for cached diagnostics of a workspace file.
type LspDiagnosticsRequest struct {
	Path string `json:"path"`
}

// LspDiagnostic is one problem rendered for the TUI.
type LspDiagnostic struct {
	Line         int    `json:"line"`
	Character    int    `json:"character"`
	EndLine      int    `json:"end_line"`
	EndCharacter int    `json:"end_character"`
	Severity     int    `json:"severity"`
	Message      string `json:"message"`
}

// LspDiagnosticsResult carries cached diagnostics, possibly empty.
type LspDiagnosticsResult struct {
	Items []LspDiagnostic `json:"items"`
}
