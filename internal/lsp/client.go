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
	conn := NewConn(stream, nil)
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
	return nil
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
