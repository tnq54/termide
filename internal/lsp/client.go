package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type Client struct {
	binary string
	args   []string

	mu     sync.Mutex
	cmd    *exec.Cmd
	conn   *Conn
	root   string
	opened map[string]bool
	diags  map[string][]Diagnostic
}

// Diagnostic is one published problem attached to a URI, flattened to
// zero-based line/character offsets.
type Diagnostic struct {
	Line         int
	Character    int
	EndLine      int
	EndCharacter int
	Severity     int
	Message      string
}

type rawDiagnostic struct {
	Range struct {
		Start struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"start"`
		End struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"end"`
	} `json:"range"`
	Severity int    `json:"severity"`
	Message  string `json:"message"`
}

type publishParams struct {
	URI         string          `json:"uri"`
	Diagnostics []rawDiagnostic `json:"diagnostics"`
}

func NewClient(binary string, args []string) *Client {
	return &Client{binary: binary, args: append([]string(nil), args...), opened: make(map[string]bool)}
}

func (c *Client) Start(ctx context.Context, root string) error {
	if c.binary == "" {
		return fmt.Errorf("lsp binary is required")
	}
	if root == "" {
		return fmt.Errorf("workspace root is required")
	}
	cmd := exec.CommandContext(ctx, c.binary, c.args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("start lsp: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("start lsp: %w", err)
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("start lsp: %w", err)
	}
	stream := &pipeStream{reader: stdout, writer: stdin, closer: stdin}
	conn := NewConn(stream, c.onNotify)
	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := conn.Call(initCtx, "initialize", map[string]any{
		"processId":    nil,
		"rootUri":      "file://" + root,
		"capabilities": map[string]any{},
	}); err != nil {
		conn.Close()
		cmd.Process.Kill()
		return fmt.Errorf("lsp initialize: %w", err)
	}
	if err := conn.Notify("initialized", map[string]any{}); err != nil {
		conn.Close()
		cmd.Process.Kill()
		return fmt.Errorf("lsp initialized: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cmd = cmd
	c.conn = conn
	c.root = root
	c.opened = make(map[string]bool)
	if c.diags == nil {
		c.diags = make(map[string][]Diagnostic)
	}
	return nil
}

// onNotify collects textDocument/publishDiagnostics per URI.
func (c *Client) onNotify(method string, params json.RawMessage) {
	if method != "textDocument/publishDiagnostics" {
		return
	}
	var published publishParams
	if err := json.Unmarshal(params, &published); err != nil || published.URI == "" {
		return
	}
	items := make([]Diagnostic, 0, len(published.Diagnostics))
	for _, raw := range published.Diagnostics {
		items = append(items, Diagnostic{
			Line:         raw.Range.Start.Line,
			Character:    raw.Range.Start.Character,
			EndLine:      raw.Range.End.Line,
			EndCharacter: raw.Range.End.Character,
			Severity:     raw.Severity,
			Message:      raw.Message,
		})
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.diags == nil {
		c.diags = make(map[string][]Diagnostic)
	}
	c.diags[published.URI] = items
}

// Diagnostics returns the latest published problems for a URI, possibly
// empty when the server reported a clean file.
func (c *Client) Diagnostics(uri string) []Diagnostic {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Diagnostic(nil), c.diags[uri]...)
}

func (c *Client) connFor() (*Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil, fmt.Errorf("lsp client is not started")
	}
	return c.conn, nil
}

// DidOpen sends textDocument/didOpen once per URI; repeats are no-ops.
func (c *Client) DidOpen(ctx context.Context, uri, languageID, text string) error {
	if uri == "" {
		return fmt.Errorf("uri is required")
	}
	c.mu.Lock()
	if c.opened[uri] {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	conn, err := c.connFor()
	if err != nil {
		return err
	}
	err = conn.Notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": languageID, "version": 1, "text": text},
	})
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.opened[uri] = true
	c.mu.Unlock()
	return nil
}

// Hover returns the hover contents as plain text at a zero-based line and
// rune-offset character. Rune offsets are sent as-is; servers counting UTF-16
// code units may disagree on non-BMP text.
func (c *Client) Hover(ctx context.Context, uri string, line, character int) (string, error) {
	conn, err := c.connFor()
	if err != nil {
		return "", err
	}
	raw, err := conn.Call(ctx, "textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": character},
	})
	if err != nil {
		return "", err
	}
	if string(raw) == "null" || len(raw) == 0 {
		return "", nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString, nil
	}
	var markup struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &markup); err == nil && markup.Value != "" {
		return markup.Value, nil
	}
	var wrapper struct {
		Contents json.RawMessage `json:"contents"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return "", fmt.Errorf("decode hover: %w", err)
	}
	if err := json.Unmarshal(wrapper.Contents, &asString); err == nil {
		return asString, nil
	}
	if err := json.Unmarshal(wrapper.Contents, &markup); err == nil && markup.Value != "" {
		return markup.Value, nil
	}
	return string(wrapper.Contents), nil
}

// CompletionItem is one completion suggestion with display and insert text.
type CompletionItem struct {
	Label  string
	Kind   int
	Detail string
	Insert string
}

type rawCompletionItem struct {
	Label      string `json:"label"`
	Kind       int    `json:"kind"`
	Detail     string `json:"detail"`
	InsertText string `json:"insertText"`
	TextEdit   *struct {
		NewText string `json:"newText"`
	} `json:"textEdit"`
}

func (r rawCompletionItem) toItem() CompletionItem {
	insert := r.InsertText
	if insert == "" && r.TextEdit != nil {
		insert = r.TextEdit.NewText
	}
	if insert == "" {
		insert = r.Label
	}
	return CompletionItem{Label: r.Label, Kind: r.Kind, Detail: r.Detail, Insert: insert}
}

// Complete asks for completions at a zero-based line and rune-offset character.
func (c *Client) Complete(ctx context.Context, uri string, line, character int) ([]CompletionItem, error) {
	conn, err := c.connFor()
	if err != nil {
		return nil, err
	}
	raw, err := conn.Call(ctx, "textDocument/completion", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": character},
	})
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" || len(raw) == 0 {
		return nil, nil
	}
	var list struct {
		Items []rawCompletionItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decode completion: %w", err)
	}
	rawItems := list.Items
	if rawItems == nil {
		var flat []rawCompletionItem
		if err := json.Unmarshal(raw, &flat); err != nil {
			return nil, fmt.Errorf("decode completion: %w", err)
		}
		rawItems = flat
	}
	items := make([]CompletionItem, 0, len(rawItems))
	for _, rawItem := range rawItems {
		items = append(items, rawItem.toItem())
	}
	return items, nil
}

// Close shuts the server down gracefully, then kills leftovers.
func (c *Client) Close() error {
	c.mu.Lock()
	conn, cmd := c.conn, c.cmd
	c.conn = nil
	c.cmd = nil
	c.mu.Unlock()
	if conn != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = conn.Call(ctx, "shutdown", nil)
		cancel()
		_ = conn.Notify("exit", nil)
		conn.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
	return nil
}

// Stop kills the server process without a graceful shutdown handshake.
func (c *Client) Stop() error {
	c.mu.Lock()
	cmd := c.cmd
	c.cmd = nil
	c.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

type pipeStream struct {
	reader io.Reader
	writer io.Writer
	closer io.Closer
}

func (s *pipeStream) Read(p []byte) (int, error)  { return s.reader.Read(p) }
func (s *pipeStream) Write(p []byte) (int, error) { return s.writer.Write(p) }
func (s *pipeStream) Close() error                { return s.closer.Close() }
