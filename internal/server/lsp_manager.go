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
	actions map[string]*cachedAction
	nonce   uint64
}

type cachedAction struct {
	workspaceID string
	root        string
	edits       map[string][]lsp.TextEdit
	created     time.Time
}

func NewLSPManager(binary string, args []string) *LSPManager {
	return &LSPManager{servers: make(map[string]*lsp.Client), binary: binary, args: append([]string(nil), args...), actions: make(map[string]*cachedAction)}
}

// CodeAction lists applicable fixes/refactors for a line range. Only actions
// carrying edits are returned; command-only actions cannot be applied
// headlessly and are dropped. Each listed action gets a single-use ID.
// A non-empty only restricts to the given action kinds (e.g. ["source"]).
func (m *LSPManager) CodeAction(ctx context.Context, workspaceID, root, rel string, startLine, startChar, endLine, endChar int, only []string) ([]protocol.LspCodeActionItem, error) {
	if workspaceID == "" || root == "" {
		return nil, fmt.Errorf("workspace identity is required")
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".py") {
		return nil, fmt.Errorf("no language server for this file type")
	}
	if startLine < 0 || startChar < 0 || endLine < startLine {
		return nil, fmt.Errorf("line range is invalid")
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
	actions, err := client.CodeAction(ctx, uri, startLine, startChar, endLine, endChar, client.Diagnostics(uri), only)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, cached := range m.actions {
		if now.Sub(cached.created) > 5*time.Minute {
			delete(m.actions, id)
		}
	}
	if len(m.actions) >= 32 {
		m.actions = make(map[string]*cachedAction)
	}
	m.nonce++
	items := make([]protocol.LspCodeActionItem, 0, len(actions))
	for i, action := range actions {
		if len(action.Edits) == 0 {
			continue
		}
		id := fmt.Sprintf("act-%d-%d", m.nonce, i)
		edits := make(map[string][]lsp.TextEdit, len(action.Edits))
		for uri, e := range action.Edits {
			edits[uri] = append([]lsp.TextEdit(nil), e...)
		}
		items = append(items, protocol.LspCodeActionItem{ID: id, Title: action.Title, Kind: action.Kind})
		m.actions[id] = &cachedAction{workspaceID: workspaceID, root: root, edits: edits, created: now}
	}
	return items, nil
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

// Complete returns completion items for a workspace-relative Python file.
func (m *LSPManager) Complete(ctx context.Context, workspaceID, root, rel string, line, character int) ([]protocol.LspCompleteItem, error) {
	if workspaceID == "" || root == "" {
		return nil, fmt.Errorf("workspace identity is required")
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".py") {
		return nil, fmt.Errorf("no language server for this file type")
	}
	if line < 0 || character < 0 {
		return nil, fmt.Errorf("line and character must be non-negative")
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
	items, err := client.Complete(ctx, uri, line, character)
	if err != nil {
		return nil, err
	}
	out := make([]protocol.LspCompleteItem, 0, len(items))
	for _, item := range items {
		out = append(out, protocol.LspCompleteItem{
			Label: item.Label, Kind: item.Kind, Detail: item.Detail, Insert: item.Insert,
		})
	}
	return out, nil
}

// SignatureHelp returns overloads for a workspace-relative Python file.
func (m *LSPManager) SignatureHelp(ctx context.Context, workspaceID, root, rel string, line, character int) (protocol.LspSignatureResult, error) {
	if workspaceID == "" || root == "" {
		return protocol.LspSignatureResult{}, fmt.Errorf("workspace identity is required")
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".py") {
		return protocol.LspSignatureResult{}, fmt.Errorf("no language server for this file type")
	}
	if line < 0 || character < 0 {
		return protocol.LspSignatureResult{}, fmt.Errorf("line and character must be non-negative")
	}
	clean, err := checkPathInsideRoot(root, rel)
	if err != nil {
		return protocol.LspSignatureResult{}, err
	}
	text, err := os.ReadFile(clean)
	if err != nil {
		return protocol.LspSignatureResult{}, err
	}
	client, err := m.serverFor(ctx, root)
	if err != nil {
		return protocol.LspSignatureResult{}, err
	}
	uri := "file://" + clean
	if err := client.DidOpen(ctx, uri, "python", string(text)); err != nil {
		return protocol.LspSignatureResult{}, err
	}
	help, err := client.SignatureHelp(ctx, uri, line, character)
	if err != nil {
		return protocol.LspSignatureResult{}, err
	}
	result := protocol.LspSignatureResult{ActiveSignature: help.ActiveSignature, ActiveParameter: help.ActiveParameter}
	for _, sig := range help.Signatures {
		entry := protocol.LspSignature{Label: sig.Label, Documentation: sig.Documentation}
		for _, param := range sig.Parameters {
			entry.Parameters = append(entry.Parameters, protocol.LspSignatureParameter{Label: param.Label})
		}
		result.Signatures = append(result.Signatures, entry)
	}
	return result, nil
}

// Rename asks the server for a workspace edit and applies it to every
// affected file inside the workspace. Files are read, validated and written
// one at a time; a file whose edits fail validation aborts the rename before
// anything is written.
func (m *LSPManager) Rename(ctx context.Context, workspaceID, root, rel string, line, character int, newName string) ([]protocol.LspRenameFile, error) {
	if workspaceID == "" || root == "" {
		return nil, fmt.Errorf("workspace identity is required")
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".py") {
		return nil, fmt.Errorf("no language server for this file type")
	}
	if line < 0 || character < 0 {
		return nil, fmt.Errorf("line and character must be non-negative")
	}
	if !isIdentifier(newName) {
		return nil, fmt.Errorf("invalid new name %q", newName)
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
	changes, err := client.Rename(ctx, uri, line, character, newName)
	if err != nil {
		return nil, err
	}
	return applyChanges(root, changes)
}

// applyChanges validates every edit, then writes all files. Nothing is
// written unless every file validates, so a bad edit cannot half-apply.
func applyChanges(root string, changes map[string][]lsp.TextEdit) ([]protocol.LspRenameFile, error) {
	type pendingFile struct {
		path string
		text string
	}
	pending := make([]pendingFile, 0, len(changes))
	files := make([]protocol.LspRenameFile, 0, len(changes))
	for fileURI, edits := range changes {
		path, err := uriWorkspaceRel(resolvedRootOf(root), fileURI)
		if err != nil {
			return nil, err
		}
		original, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		updated, err := applyTextEdits(string(original), edits)
		if err != nil {
			return nil, fmt.Errorf("file %s: %w", relOf(root, path), err)
		}
		pending = append(pending, pendingFile{path: path, text: updated})
		files = append(files, protocol.LspRenameFile{Path: relOf(root, path), Edits: len(edits)})
	}
	for _, file := range pending {
		if err := os.WriteFile(file.path, []byte(file.text), 0600); err != nil {
			return nil, err
		}
	}
	sortRenameFiles(files)
	return files, nil
}

// ApplyCodeAction applies a previously listed action and consumes its nonce.
// Unknown or reused IDs are rejected.
func (m *LSPManager) ApplyCodeAction(workspaceID, id string) ([]protocol.LspRenameFile, error) {
	m.mu.Lock()
	cached, ok := m.actions[id]
	if ok {
		delete(m.actions, id)
	}
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("unknown or expired code action")
	}
	if cached.workspaceID != workspaceID {
		return nil, fmt.Errorf("code action belongs to another workspace")
	}
	return applyChanges(cached.root, cached.edits)
}

func isIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func resolvedRootOf(root string) string {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return root
	}
	return resolved
}

func relOf(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

// uriWorkspaceRel maps a file:// URI back into the workspace, rejecting
// URIs that escape the workspace root.
func uriWorkspaceRel(resolvedRoot, uri string) (string, error) {
	const prefix = "file://"
	if !strings.HasPrefix(uri, prefix) {
		return "", fmt.Errorf("unsupported uri %q", uri)
	}
	clean := filepath.Clean(strings.TrimPrefix(uri, prefix))
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		parent, perr := filepath.EvalSymlinks(filepath.Dir(clean))
		if perr != nil {
			return "", err
		}
		resolved = filepath.Join(parent, filepath.Base(clean))
	}
	if resolved != resolvedRoot && !strings.HasPrefix(resolved, resolvedRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("rename target escapes workspace root")
	}
	return resolved, nil
}

// applyTextEdits splices non-overlapping edits into text. Offsets are rune
// based; edits are applied from the end of the file backwards so earlier
// edits never shift later offsets.
func applyTextEdits(text string, edits []lsp.TextEdit) (string, error) {
	lines := strings.SplitAfter(text, "\n")
	starts := make([]int, len(lines)+1)
	for i, line := range lines {
		starts[i+1] = starts[i] + len([]rune(line))
	}
	type op struct {
		start, end int
		newText    string
	}
	ops := make([]op, 0, len(edits))
	for _, edit := range edits {
		if edit.Start.Line < 0 || edit.End.Line < edit.Start.Line || len(lines) == 0 {
			return "", fmt.Errorf("edit out of range")
		}
		if edit.Start.Line >= len(lines) || edit.End.Line >= len(lines) {
			return "", fmt.Errorf("edit out of range")
		}
		startLen := len([]rune(strings.TrimSuffix(lines[edit.Start.Line], "\n")))
		endLen := len([]rune(strings.TrimSuffix(lines[edit.End.Line], "\n")))
		if edit.Start.Character < 0 || edit.Start.Character > startLen {
			return "", fmt.Errorf("edit out of range")
		}
		if edit.End.Character < 0 || edit.End.Character > endLen {
			return "", fmt.Errorf("edit out of range")
		}
		start := starts[edit.Start.Line] + edit.Start.Character
		end := starts[edit.End.Line] + edit.End.Character
		if end < start {
			return "", fmt.Errorf("edit out of range")
		}
		ops = append(ops, op{start, end, edit.NewText})
	}
	for i := 1; i < len(ops); i++ {
		j := i
		for j > 0 && ops[j].start > ops[j-1].start {
			ops[j], ops[j-1] = ops[j-1], ops[j]
			j--
		}
	}
	runes := []rune(text)
	for i, op := range ops {
		if i > 0 && op.end > ops[i-1].start {
			return "", fmt.Errorf("overlapping edits")
		}
		replacement := []rune(op.newText)
		kept := append([]rune(nil), runes[:op.start]...)
		kept = append(kept, replacement...)
		kept = append(kept, runes[op.end:]...)
		runes = kept
	}
	return string(runes), nil
}

func sortRenameFiles(files []protocol.LspRenameFile) {
	for i := 1; i < len(files); i++ {
		j := i
		for j > 0 && files[j].Path < files[j-1].Path {
			files[j], files[j-1] = files[j-1], files[j]
			j--
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
