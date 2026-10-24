package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const fakeLspDiagnostics = `import json,sys
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
        write_msg({"jsonrpc": "2.0", "method": "textDocument/publishDiagnostics", "params": {"uri": uri, "diagnostics": [{"range": {"start": {"line": 1, "character": 4}, "end": {"line": 1, "character": 7}}, "severity": 1, "message": "undefined name 'foo'"}]}})
    elif method == "shutdown":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "exit":
        break
`

func TestClientCollectsPublishedDiagnostics(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspDiagnostics), 0700); err != nil {
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
	if err := client.DidOpen(ctx, uri, "python", "x = 1\ny = foo\n"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		items := client.Diagnostics(uri)
		if len(items) == 1 && items[0].Line == 1 && items[0].Message == "undefined name 'foo'" {
			if items[0].Severity != 1 {
				t.Fatalf("severity = %d", items[0].Severity)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("diagnostics = %#v", client.Diagnostics(uri))
		}
		time.Sleep(50 * time.Millisecond)
	}
}
