package lsp

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestJSONRPCFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := Message{JSONRPC: "2.0", ID: 1, Method: "textDocument/completion", Params: json.RawMessage(`{"x":1}`)}
	if err := Write(&buf, want); err != nil {
		t.Fatal(err)
	}
	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != want.Method || got.JSONRPC != "2.0" {
		t.Fatalf("got %#v", got)
	}
}
