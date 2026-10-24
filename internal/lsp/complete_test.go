package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
        write_msg({"jsonrpc": "2.0", "id": mid, "result": {"items": [{"label": "append", "kind": 2, "detail": "list.append", "insertText": "append($0)"}, {"label": "abs", "kind": 3, "detail": "builtin"}]}})
    elif method == "shutdown":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "exit":
        break
`

func TestClientCompleteEndToEnd(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspComplete), 0700); err != nil {
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
	if err := client.DidOpen(ctx, uri, "python", "x = []."); err != nil {
		t.Fatal(err)
	}
	items, err := client.Complete(ctx, uri, 0, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %#v", items)
	}
	if items[0].Label != "append" || items[0].Kind != 2 || items[0].Detail != "list.append" || items[0].Insert != "append($0)" {
		t.Fatalf("items[0] = %#v", items[0])
	}
	if items[1].Label != "abs" || items[1].Insert != "abs" {
		t.Fatalf("items[1] fallback insert = %#v", items[1])
	}
}
