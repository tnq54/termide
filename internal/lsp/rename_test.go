package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const fakeLspRename = `import json,sys
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
    elif method == "textDocument/rename":
        uri = msg["params"]["textDocumentUri"] if "textDocumentUri" in msg["params"] else msg["params"]["textDocument"]["uri"]
        write_msg({"jsonrpc": "2.0", "id": mid, "result": {"changes": {uri: [{"range": {"start": {"line": 0, "character": 4}, "end": {"line": 0, "character": 7}}, "newText": "total"}]}}})
    elif method == "shutdown":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "exit":
        break
`

func TestClientRenameEndToEnd(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspRename), 0700); err != nil {
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
	changes, err := client.Rename(ctx, uri, 0, 4, "total")
	if err != nil {
		t.Fatal(err)
	}
	edits, ok := changes[uri]
	if !ok || len(edits) != 1 {
		t.Fatalf("changes = %#v", changes)
	}
	if edits[0].NewText != "total" || edits[0].Start.Line != 0 || edits[0].Start.Character != 4 || edits[0].End.Character != 7 {
		t.Fatalf("edit = %#v", edits[0])
	}
}

const fakeLspRenameNull = `import json,sys
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
    elif method == "textDocument/rename":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "shutdown":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "exit":
        break
`

func TestClientRenameNullResult(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspRenameNull), 0700); err != nil {
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
	changes, err := client.Rename(ctx, uri, 0, 0, "total")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("changes = %#v", changes)
	}
}
