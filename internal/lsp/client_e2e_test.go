package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fakeLspServer = `import json,sys
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

func TestClientHoverEndToEnd(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspServer), 0700); err != nil {
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
	if err := client.DidOpen(ctx, uri, "python", "x = len([1])\n"); err != nil {
		t.Fatal(err)
	}
	if err := client.DidOpen(ctx, uri, "python", "x = len([1])\n"); err != nil {
		t.Fatal("duplicate didOpen failed")
	}
	contents, err := client.Hover(ctx, uri, 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(contents, "len") {
		t.Fatalf("contents = %q", contents)
	}
}
