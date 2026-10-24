package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func Write(w io.Writer, message Message) error {
	if message.JSONRPC == "" {
		message.JSONRPC = "2.0"
	}
	if message.JSONRPC == "" || (message.Method == "" && message.ID == nil) {
		return fmt.Errorf("lsp method or ID is required")
	}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func Read(r io.Reader) (Message, error) {
	header := bufio.NewReader(r)
	length := 0
	for {
		line, err := header.ReadString('\n')
		if err != nil {
			return Message{}, err
		}
		line = string(bytes.TrimSpace([]byte(line)))
		if line == "" {
			break
		}
		var n int
		if _, err := fmt.Sscanf(line, "Content-Length: %d", &n); err == nil {
			length = n
		}
	}
	if length <= 0 || length > 16*1024*1024 {
		return Message{}, fmt.Errorf("invalid content length %d", length)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(header, data); err != nil {
		return Message{}, err
	}
	var message Message
	if err := json.Unmarshal(data, &message); err != nil {
		return Message{}, err
	}
	if message.JSONRPC == "" || (message.Method == "" && message.ID == nil) {
		return Message{}, fmt.Errorf("invalid lsp frame")
	}
	return message, nil
}
