package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const fakeLspCodeAction = `import json,sys
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
    elif method == "textDocument/codeAction":
        uri = msg["params"]["textDocument"]["uri"]
        write_msg({"jsonrpc": "2.0", "id": mid, "result": [{"title": "Add missing import", "kind": "quickfix", "edit": {"changes": {uri: [{"range": {"start": {"line": 0, "character": 0}, "end": {"line": 0, "character": 0}}, "newText": "import os\n"}]}}}, {"title": "Run fixer", "kind": "quickfix", "command": {"title": "run", "command": "fix"}}]})
    elif method == "shutdown":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "exit":
        break
`

func TestClientCodeActionEndToEnd(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspCodeAction), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := NewClient("python3", []string{script})
	if err := client.Start(ctx, root); err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	uri := "file://" + filepath.Join(root, "x.py")
	actions, err := client.CodeAction(ctx, uri, 1, 0, 1, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 {
		t.Fatalf("actions = %#v", actions)
	}
	if actions[0].Title != "Add missing import" || actions[0].Kind != "quickfix" {
		t.Fatalf("actions[0] = %#v", actions[0])
	}
	edits := actions[0].Edits[uri]
	if len(edits) != 1 || edits[0].NewText != "import os\n" {
		t.Fatalf("edits = %#v", actions[0].Edits)
	}
	if actions[1].Title != "Run fixer" || len(actions[1].Edits) != 0 {
		t.Fatalf("actions[1] = %#v", actions[1])
	}
}

const fakeLspCodeActionNull = `import json,sys
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
    elif method == "textDocument/codeAction":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "shutdown":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "exit":
        break
`

func TestClientCodeActionNullResult(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspCodeActionNull), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := NewClient("python3", []string{script})
	if err := client.Start(ctx, root); err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	uri := "file://" + filepath.Join(root, "x.py")
	actions, err := client.CodeAction(ctx, uri, 0, 0, 0, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 {
		t.Fatalf("actions = %#v", actions)
	}
}
