package protocol

const (
	MessageLspHover             MessageType = "lsp.hover"
	MessageLspHoverResult       MessageType = "lsp.hover_result"
	MessageLspDiagnostics       MessageType = "lsp.diagnostics"
	MessageLspDiagnosticsResult MessageType = "lsp.diagnostics_result"
	MessageLspComplete          MessageType = "lsp.complete"
	MessageLspCompleteResult    MessageType = "lsp.complete_result"
	MessageLspSignature         MessageType = "lsp.signature"
	MessageLspSignatureResult   MessageType = "lsp.signature_result"
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

// LspCompleteRequest asks for completion items at a zero-based line and
// rune-offset character in a workspace-relative file.
type LspCompleteRequest struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
}

// LspCompleteItem is one completion suggestion rendered for the TUI.
type LspCompleteItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind"`
	Detail string `json:"detail"`
	Insert string `json:"insert"`
}

// LspCompleteResult carries completion items, possibly empty.
type LspCompleteResult struct {
	Items []LspCompleteItem `json:"items"`
}

// LspSignatureRequest asks for signature help at a zero-based line and
// rune-offset character in a workspace-relative file.
type LspSignatureRequest struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
}

// LspSignatureParameter is one parameter label of a signature.
type LspSignatureParameter struct {
	Label string `json:"label"`
}

// LspSignature is one overload rendered for the TUI.
type LspSignature struct {
	Label         string                  `json:"label"`
	Documentation string                  `json:"documentation"`
	Parameters    []LspSignatureParameter `json:"parameters"`
}

// LspSignatureResult carries overloads plus the active indices.
type LspSignatureResult struct {
	Signatures      []LspSignature `json:"signatures"`
	ActiveSignature int            `json:"active_signature"`
	ActiveParameter int            `json:"active_parameter"`
}
