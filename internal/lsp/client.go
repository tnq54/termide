package lsp

import (
	"context"
	"fmt"
	"os/exec"
)

type Client struct { binary string; args []string; cmd *exec.Cmd }
func NewClient(binary string, args []string) *Client { return &Client{binary: binary, args: append([]string(nil), args...)} }
func (c *Client) Start(ctx context.Context) error {
	if c.binary == "" { return fmt.Errorf("lsp binary is required") }
	c.cmd = exec.CommandContext(ctx, c.binary, c.args...)
	if err := c.cmd.Start(); err != nil { return fmt.Errorf("start lsp: %w", err) }
	return nil
}
func (c *Client) Stop() error { if c.cmd == nil || c.cmd.Process == nil { return nil }; return c.cmd.Process.Kill() }
