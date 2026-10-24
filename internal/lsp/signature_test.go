package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
        write_msg({"jsonrpc": "2.0", "id": mid, "result": {"capabilities": {}}})
    elif method == "textDocument/signatureHelp":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": {"signatures": [{"label": "len(obj, /)", "documentation": "Return the number of items.", "parameters": [{"label": "obj"}]}, {"label": "len(*args)", "parameters": []}], "activeSignature": 1, "activeParameter": 0}})
    elif method == "shutdown":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "exit":
        break
`

func TestClientSignatureHelpEndToEnd(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspSignature), 0700); err != nil {
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
	if err := client.DidOpen(ctx, uri, "python", "x = len("); err != nil {
		t.Fatal(err)
	}
	help, err := client.SignatureHelp(ctx, uri, 0, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(help.Signatures) != 2 {
		t.Fatalf("help = %#v", help)
	}
	if help.Signatures[0].Label != "len(obj, /)" || help.Signatures[0].Documentation != "Return the number of items." {
		t.Fatalf("sig[0] = %#v", help.Signatures[0])
	}
	if len(help.Signatures[0].Parameters) != 1 || help.Signatures[0].Parameters[0].Label != "obj" {
		t.Fatalf("params = %#v", help.Signatures[0])
	}
	if help.ActiveSignature != 1 || help.ActiveParameter != 0 {
		t.Fatalf("active = %d/%d", help.ActiveSignature, help.ActiveParameter)
	}
}

const fakeLspSignatureNull = `import json,sys
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
    elif method == "textDocument/signatureHelp":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "shutdown":
        write_msg({"jsonrpc": "2.0", "id": mid, "result": None})
    elif method == "exit":
        break
`

func TestClientSignatureHelpNullResult(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "fake-lsp.py")
	if err := os.WriteFile(script, []byte(fakeLspSignatureNull), 0700); err != nil {
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
	help, err := client.SignatureHelp(ctx, uri, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if help == nil || len(help.Signatures) != 0 {
		t.Fatalf("help = %#v", help)
	}
}
