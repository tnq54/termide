package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

const fakeLspPython = `import json,sys
def read_msg():
    length = 0
    while True:
        line = sys.stdin.buffer.readline().decode()
        if not line or line in ("\r\n", "\n"):
            break
        if line.lower().startswith("content-length:"):
            length = int(line.split(":")[1].strip())
    return json.loads(sys.stdin.buffer.read(length).decode()) if length else None
def write_msg(obj):
    data = json.dumps(obj).encode()
    sys.stdout.buffer.write(b"Content-Length: %d\r\n\r\n" % len(data) + data)
    sys.stdout.buffer.flush()
while True:
    msg = read_msg()
    if msg is None:
        break
    method = msg.get("method", "")
    mid = msg.get("id")
    if method == "initialize":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": {"capabilities": {}}})
    elif method == "textDocument/hover":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": {"contents": {"kind": "markdown", "value": "**len** builtin"}}})
    elif method == "shutdown":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "exit":
        break
`

func lspHandlerFixture(t *testing.T) (*Handler, context.Context) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	script := filepath.Join(t.TempDir(), "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspPython), 0700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "x.py"), []byte("x = len([1])\n"), 0600); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(s)
	h.lsp = NewLSPManager("python3", []string{script})
	h.lspOwned = true
	return h, ctx
}

func lspHover(t *testing.T, h *Handler, ctx context.Context, path string, line, char int) string {
	t.Helper()
	raw, err := json.Marshal(protocol.LspHoverRequest{Path: path, Line: line, Character: char})
	if err != nil {
		t.Fatal(err)
	}
	response, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageLspHover, WorkspaceID: "w", Payload: raw})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageLspHoverResult || response.WorkspaceID != "w" {
		t.Fatalf("response = %#v", response)
	}
	var result protocol.LspHoverResult
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		t.Fatal(err)
	}
	return result.Contents
}

func TestLspHoverEndToEnd(t *testing.T) {
	h, ctx := lspHandlerFixture(t)
	contents := lspHover(t, h, ctx, "x.py", 0, 4)
	if !strings.Contains(contents, "len") {
		t.Fatalf("contents = %q", contents)
	}
}

func TestLspHoverRejectsNonPython(t *testing.T) {
	h, ctx := lspHandlerFixture(t)
	raw, _ := json.Marshal(protocol.LspHoverRequest{Path: "x.go", Line: 0, Character: 0})
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageLspHover, WorkspaceID: "w", Payload: raw}); err == nil {
		t.Fatal("non-python file accepted")
	}
}

func TestLspHoverRejectsEscape(t *testing.T) {
	h, ctx := lspHandlerFixture(t)
	raw, _ := json.Marshal(protocol.LspHoverRequest{Path: "../evil.py", Line: 0, Character: 0})
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageLspHover, WorkspaceID: "w", Payload: raw}); err == nil {
		t.Fatal("path escape accepted")
	}
}

const fakeLspComplete = `import json,sys
def read_msg():
    length = 0
    while True:
        line = sys.stdin.buffer.readline().decode()
        if not line or line in ("\r\n", "\n"):
            break
        if line.lower().startswith("content-length:"):
            length = int(line.split(":")[1].strip())
    return json.loads(sys.stdin.buffer.read(length).decode()) if length else None
def write_msg(obj):
    data = json.dumps(obj).encode()
    sys.stdout.buffer.write(b"Content-Length: %d\r\n\r\n" % len(data) + data)
    sys.stdout.buffer.flush()
while True:
    msg = read_msg()
    if msg is None:
        break
    method = msg.get("method", "")
    mid = msg.get("id")
    if method == "initialize":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": {"capabilities": {}}})
    elif method == "textDocument/completion":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": {"items": [{"label": "append", "kind": 2, "detail": "list.append"}, {"label": "abs", "kind": 3, "detail": "builtin", "insertText": "abs()"}]}})
    elif method == "shutdown":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "exit":
        break
`

func TestLspCompleteEndToEnd(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	script := filepath.Join(t.TempDir(), "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspComplete), 0700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "x.py"), []byte("x = []."), 0600); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(s)
	defer h.Close()
	h.lsp = NewLSPManager("python3", []string{script})
	h.lspOwned = true
	raw, _ := json.Marshal(protocol.LspCompleteRequest{Path: "x.py", Line: 0, Character: 7})
	response, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageLspComplete, WorkspaceID: "w", Payload: raw})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageLspCompleteResult {
		t.Fatalf("response = %#v", response)
	}
	var result protocol.LspCompleteResult
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 || result.Items[0].Label != "append" {
		t.Fatalf("result = %#v", result)
	}
	if result.Items[1].Insert != "abs()" {
		t.Fatalf("insert fallback = %#v", result.Items[1])
	}
}

func TestLspCompleteRejectsNonPython(t *testing.T) {
	h, ctx := lspHandlerFixture(t)
	raw, _ := json.Marshal(protocol.LspCompleteRequest{Path: "x.go", Line: 0, Character: 0})
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageLspComplete, WorkspaceID: "w", Payload: raw}); err == nil {
		t.Fatal("non-python file accepted")
	}
}

const fakeLspSignature = `import json,sys
def read_msg():
    length = 0
    while True:
        line = sys.stdin.buffer.readline().decode()
        if not line or line in ("\r\n", "\n"):
            break
        if line.lower().startswith("content-length:"):
            length = int(line.split(":")[1].strip())
    return json.loads(sys.stdin.buffer.read(length).decode()) if length else None
def write_msg(obj):
    data = json.dumps(obj).encode()
    sys.stdout.buffer.write(b"Content-Length: %d\r\n\r\n" % len(data) + data)
    sys.stdout.buffer.flush()
while True:
    msg = read_msg()
    if msg is None:
        break
    method = msg.get("method", "")
    mid = msg.get("id")
    if method == "initialize":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": {"capabilities": {"signatureHelpProvider": {}}}})
    elif method == "textDocument/signatureHelp":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": {"signatures": [{"label": "len(obj, /)", "documentation": "Return the number of items.", "parameters": [{"label": "obj"}]}], "activeSignature": 0, "activeParameter": 0}})
    elif method == "shutdown":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "exit":
        break
`

func TestLspSignatureEndToEnd(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	script := filepath.Join(t.TempDir(), "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspSignature), 0700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "x.py"), []byte("x = len("), 0600); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(s)
	defer h.Close()
	h.lsp = NewLSPManager("python3", []string{script})
	h.lspOwned = true
	raw, _ := json.Marshal(protocol.LspSignatureRequest{Path: "x.py", Line: 0, Character: 7})
	response, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageLspSignature, WorkspaceID: "w", Payload: raw})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageLspSignatureResult {
		t.Fatalf("response = %#v", response)
	}
	var result protocol.LspSignatureResult
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Signatures) != 1 || result.Signatures[0].Label != "len(obj, /)" {
		t.Fatalf("result = %#v", result)
	}
	if result.Signatures[0].Documentation != "Return the number of items." {
		t.Fatalf("result = %#v", result)
	}
}

func TestLspSignatureRejectsNonPython(t *testing.T) {
	h, ctx := lspHandlerFixture(t)
	raw, _ := json.Marshal(protocol.LspSignatureRequest{Path: "x.go", Line: 0, Character: 0})
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageLspSignature, WorkspaceID: "w", Payload: raw}); err == nil {
		t.Fatal("non-python file accepted")
	}
}

func TestLspHoverRequiresConfiguredServer(t *testing.T) {
	h, ctx := lspHandlerFixture(t)
	h.lsp = nil
	raw, _ := json.Marshal(protocol.LspHoverRequest{Path: "x.py", Line: 0, Character: 0})
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageLspHover, WorkspaceID: "w", Payload: raw}); err == nil {
		t.Fatal("unconfigured server accepted")
	}
}

const fakeLspPublishing = `import json,sys
def read_msg():
    length = 0
    while True:
        line = sys.stdin.buffer.readline().decode()
        if not line or line in ("\r\n", "\n"):
            break
        if line.lower().startswith("content-length:"):
            length = int(line.split(":")[1].strip())
    return json.loads(sys.stdin.buffer.read(length).decode()) if length else None
def write_msg(obj):
    data = json.dumps(obj).encode()
    sys.stdout.buffer.write(b"Content-Length: %d\r\n\r\n" % len(data) + data)
    sys.stdout.buffer.flush()
while True:
    msg = read_msg()
    if msg is None:
        break
    method = msg.get("method", "")
    mid = msg.get("id")
    if method == "initialize":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": {"capabilities": {}}})
    elif method == "textDocument/didOpen":
        uri = msg["params"]["textDocument"]["uri"]
        write_msg({"jsonrpc": "2.0", "method": "textDocument/publishDiagnostics", "params": {"uri": uri, "diagnostics": [{"range": {"start": {"line": 0, "character": 4}, "end": {"line": 0, "character": 7}}, "severity": 1, "message": "undefined name"}]}})
    elif method == "shutdown":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "exit":
        break
`

func TestLspDiagnosticsEndToEnd(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	script := filepath.Join(t.TempDir(), "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspPublishing), 0700); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "x.py"), []byte("x = foo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(s)
	defer h.Close()
	h.lsp = NewLSPManager("python3", []string{script})
	h.lspOwned = true
	raw, _ := json.Marshal(protocol.LspDiagnosticsRequest{Path: "x.py"})
	response, err := h.Handle(ctx, protocol.Message{Type: protocol.MessageLspDiagnostics, WorkspaceID: "w", Payload: raw})
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageLspDiagnosticsResult {
		t.Fatalf("response = %#v", response)
	}
	var result protocol.LspDiagnosticsResult
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Message != "undefined name" {
		t.Fatalf("result = %#v", result)
	}
}
