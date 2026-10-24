package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"termide/internal/lsp"
	"termide/internal/protocol"
)

// LSPManager keeps one language server per workspace root, started on demand
// from the configured Python server binary. Daemon-scoped like the PTY
// registry: servers survive reconnects and die with the supervisor.
type LSPManager struct {
	mu      sync.Mutex
	servers map[string]*lsp.Client
	binary  string
	args    []string
}

func NewLSPManager(binary string, args []string) *LSPManager {
	return &LSPManager{servers: make(map[string]*lsp.Client), binary: binary, args: append([]string(nil), args...)}
}

func (m *LSPManager) serverFor(ctx context.Context, root string) (*lsp.Client, error) {
	if m.binary == "" {
		return nil, fmt.Errorf("language server is not configured")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if client, ok := m.servers[root]; ok {
		return client, nil
	}
	client := lsp.NewClient(m.binary, m.args)
	if err := client.Start(ctx, root); err != nil {
		return nil, err
	}
	m.servers[root] = client
	return client, nil
}

// Hover returns hover text for a workspace-relative Python file. Only .py is
// supported; other extensions get a clear error instead of a meaningless
// server round-trip.
func (m *LSPManager) Hover(ctx context.Context, workspaceID, root, rel string, line, character int) (string, error) {
	if workspaceID == "" || root == "" {
		return "", fmt.Errorf("workspace identity is required")
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".py") {
		return "", fmt.Errorf("no language server for this file type")
	}
	if line < 0 || character < 0 {
		return "", fmt.Errorf("line and character must be non-negative")
	}
	clean, err := checkPathInsideRoot(root, rel)
	if err != nil {
		return "", err
	}
	text, err := os.ReadFile(clean)
	if err != nil {
		return "", err
	}
	client, err := m.serverFor(ctx, root)
	if err != nil {
		return "", err
	}
	uri := "file://" + clean
	if err := client.DidOpen(ctx, uri, "python", string(text)); err != nil {
		return "", err
	}
	return client.Hover(ctx, uri, line, character)
}

// Diagnostics ensures the file is open on the server and returns cached
// problems, waiting briefly for the server's first verdict. An empty result
// means the file is clean, not that the server is silent.
func (m *LSPManager) Diagnostics(ctx context.Context, workspaceID, root, rel string) ([]protocol.LspDiagnostic, error) {
	if workspaceID == "" || root == "" {
		return nil, fmt.Errorf("workspace identity is required")
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".py") {
		return nil, fmt.Errorf("no language server for this file type")
	}
	clean, err := checkPathInsideRoot(root, rel)
	if err != nil {
		return nil, err
	}
	text, err := os.ReadFile(clean)
	if err != nil {
		return nil, err
	}
	client, err := m.serverFor(ctx, root)
	if err != nil {
		return nil, err
	}
	uri := "file://" + clean
	if err := client.DidOpen(ctx, uri, "python", string(text)); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		items := client.Diagnostics(uri)
		if len(items) > 0 || time.Now().After(deadline) {
			out := make([]protocol.LspDiagnostic, 0, len(items))
			for _, item := range items {
				out = append(out, protocol.LspDiagnostic{
					Line: item.Line, Character: item.Character,
					EndLine: item.EndLine, EndCharacter: item.EndCharacter,
					Severity: item.Severity, Message: item.Message,
				})
			}
			return out, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func checkPathInsideRoot(root, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be workspace-relative")
	}
	joined := filepath.Join(root, filepath.Clean("/"+rel))
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		// File may not exist yet; resolve the parent instead.
		parent, perr := filepath.EvalSymlinks(filepath.Dir(joined))
		if perr != nil {
			return "", err
		}
		resolved = filepath.Join(parent, filepath.Base(joined))
	}
	if resolved != resolvedRoot && !strings.HasPrefix(resolved, resolvedRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes workspace root")
	}
	return resolved, nil
}

// Close stops every managed server.
func (m *LSPManager) Close() {
	m.mu.Lock()
	servers := make([]*lsp.Client, 0, len(m.servers))
	for root, client := range m.servers {
		servers = append(servers, client)
		delete(m.servers, root)
	}
	m.mu.Unlock()
	for _, client := range servers {
		client.Close()
	}
}
