package server

import (
	"context"
	"encoding/json"
	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
	"testing"
)

func TestOpenWorkspaceIsRepeatableAndRejectsRebinding(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := NewHandler(s)
	root := t.TempDir()
	open := func(root string) error {
		payload, _ := json.Marshal(domain.Workspace{ID: "w", Root: root, Name: "project"})
		_, err := h.Handle(context.Background(), protocol.Message{Type: protocol.MessageOpenWorkspace, Payload: payload})
		return err
	}
	if err := open(root); err != nil {
		t.Fatal(err)
	}
	if err := open(root); err != nil {
		t.Fatal(err)
	}
	if err := open(t.TempDir()); err == nil {
		t.Fatal("workspace root was rebound")
	}
	if err := open(root + "/missing"); err == nil {
		t.Fatal("nonexistent root accepted")
	}
}
