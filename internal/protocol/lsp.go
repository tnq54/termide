package protocol

const (
	MessageLspHover       MessageType = "lsp.hover"
	MessageLspHoverResult MessageType = "lsp.hover_result"
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
