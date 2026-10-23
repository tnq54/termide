package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func TestFileProtocolReadSaveAndConflict(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "main.py"), []byte("pass\n"), 0600)
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(context.Background(), domain.Workspace{ID: "w", Root: root}); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(s)
	call := func(kind protocol.MessageType, req protocol.FileRequest) (protocol.FileResponse, error) {
		payload, _ := json.Marshal(req)
		response, err := h.Handle(context.Background(), protocol.Message{Type: kind, WorkspaceID: "w", Payload: payload})
		if err != nil {
			return protocol.FileResponse{}, err
		}
		var result protocol.FileResponse
		err = json.Unmarshal(response.Payload, &result)
		return result, err
	}
	list, err := call(protocol.MessageListFiles, protocol.FileRequest{Path: "."})
	if err != nil || len(list.Entries) != 1 {
		t.Fatalf("%+v %v", list, err)
	}
	read, err := call(protocol.MessageReadFile, protocol.FileRequest{Path: "main.py"})
	if err != nil {
		t.Fatal(err)
	}
	version := read.Document.Version
	result, err := call(protocol.MessageSaveFile, protocol.FileRequest{Path: "main.py", Version: version, Text: "print('hello')\n"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document.Version == version {
		t.Fatal("version not updated")
	}
	if _, err := call(protocol.MessageSaveFile, protocol.FileRequest{Path: "main.py", Version: version, Text: "lost update"}); err == nil {
		t.Fatal("conflict not rejected")
	}
	if _, err := call(protocol.MessageReadFile, protocol.FileRequest{Path: "../outside"}); err == nil {
		t.Fatal("traversal accepted")
	}
}

func TestSaveRequiresExplicitStringText(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "main.py"), []byte("pass\n"), 0600)
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.CreateWorkspace(context.Background(), domain.Workspace{ID: "w", Root: root})
	h := NewHandler(s)
	response, err := h.Handle(context.Background(), protocol.Message{Type: protocol.MessageReadFile, WorkspaceID: "w", Payload: []byte(`{"path":"main.py"}`)})
	if err != nil {
		t.Fatal(err)
	}
	var read protocol.FileResponse
	json.Unmarshal(response.Payload, &read)
	for _, value := range []any{nil, 42, false} {
		payload, _ := json.Marshal(map[string]any{"path": "main.py", "version": read.Document.Version, "text": value})
		if _, err := h.Handle(context.Background(), protocol.Message{Type: protocol.MessageSaveFile, WorkspaceID: "w", Payload: payload}); err == nil {
			t.Fatalf("accepted text=%v", value)
		}
	}
	payload, _ := json.Marshal(map[string]any{"path": "main.py", "version": read.Document.Version})
	if _, err := h.Handle(context.Background(), protocol.Message{Type: protocol.MessageSaveFile, WorkspaceID: "w", Payload: payload}); err == nil {
		t.Fatal("accepted missing text")
	}
	data, _ := os.ReadFile(filepath.Join(root, "main.py"))
	if string(data) != "pass\n" {
		t.Fatal("bad request erased file")
	}
	payload, _ = json.Marshal(protocol.FileRequest{Path: "main.py", Version: read.Document.Version, Text: ""})
	if _, err := h.Handle(context.Background(), protocol.Message{Type: protocol.MessageSaveFile, WorkspaceID: "w", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(root, "main.py"))
	if len(data) != 0 {
		t.Fatal("intentional empty save failed")
	}
}
