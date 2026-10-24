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
	MessageLspRename            MessageType = "lsp.rename"
	MessageLspRenameResult      MessageType = "lsp.rename_result"
	MessageLspCodeAction        MessageType = "lsp.code_action"
	MessageLspCodeActionList    MessageType = "lsp.code_action_list"
	MessageLspCodeActionApply   MessageType = "lsp.code_action_apply"
	MessageLspCodeActionApplied MessageType = "lsp.code_action_applied"
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

// LspRenameRequest asks to rename the symbol at a zero-based line and
// rune-offset character in a workspace-relative file.
type LspRenameRequest struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
	NewName   string `json:"new_name"`
}

// LspRenameFile reports edits applied to one workspace-relative file.
type LspRenameFile struct {
	Path  string `json:"path"`
	Edits int    `json:"edits"`
}

// LspRenameResult carries per-file edit counts for an applied rename.
type LspRenameResult struct {
	Files []LspRenameFile `json:"files"`
}

// LspSignatureResult carries overloads plus the active indices.
type LspSignatureResult struct {
	Signatures      []LspSignature `json:"signatures"`
	ActiveSignature int            `json:"active_signature"`
	ActiveParameter int            `json:"active_parameter"`
}

// LspCodeActionRequest asks which fixes/refactors apply to a zero-based line
// range in a workspace-relative file.
type LspCodeActionRequest struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	StartChar int    `json:"start_character"`
	EndLine   int    `json:"end_line"`
	EndChar   int    `json:"end_character"`
}

// LspCodeActionItem is one applicable action. ID is an opaque server handle
// valid for a single apply call.
type LspCodeActionItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

// LspCodeActionList carries applicable actions, possibly empty.
type LspCodeActionList struct {
	Actions []LspCodeActionItem `json:"actions"`
}

// LspCodeActionApplyRequest applies a previously listed action by ID.
type LspCodeActionApplyRequest struct {
	ID string `json:"id"`
}
