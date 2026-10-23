package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID any `json:"id,omitempty"`
	Method string `json:"method"`
	Params any `json:"params,omitempty"`
}

func Write(w io.Writer, request Request) error {
	if request.JSONRPC == "" { request.JSONRPC = "2.0" }
	if request.Method == "" { return fmt.Errorf("lsp method is required") }
	data, err := json.Marshal(request); if err != nil { return err }
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data)); if err != nil { return err }
	_, err = w.Write(data); return err
}

func Read(r io.Reader) (Request, error) {
	header := bufio.NewReader(r)
	length := 0
	for {
		line, err := header.ReadString('\n'); if err != nil { return Request{}, err }
		line = string(bytes.TrimSpace([]byte(line)))
		if line == "" { break }
		var n int
		if _, err := fmt.Sscanf(line, "Content-Length: %d", &n); err == nil { length = n }
	}
	if length <= 0 || length > 16*1024*1024 { return Request{}, fmt.Errorf("invalid content length %d", length) }
	data := make([]byte, length); if _, err := io.ReadFull(header, data); err != nil { return Request{}, err }
	var request Request; if err := json.Unmarshal(data, &request); err != nil { return Request{}, err }
	if request.JSONRPC == "" || request.Method == "" { return Request{}, fmt.Errorf("invalid lsp request") }
	return request, nil
}
