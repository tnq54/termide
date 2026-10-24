package server

import (
	"context"
	"encoding/json"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func TestSupervisorHandlerServesPty(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	sup := NewSupervisor(s, nil, "owner")
	defer sup.Close(ctx)
	response, err := sup.Handler().Handle(ctx, protocol.Message{Type: protocol.MessagePtyOpen, WorkspaceID: "w", Payload: mustJSON(t, protocol.PtyOpenRequest{Rows: 24, Cols: 80})})
	if err != nil {
		t.Fatal(err)
	}
	var opened protocol.PtyOpened
	if err := json.Unmarshal(response.Payload, &opened); err != nil || opened.ID == "" {
		t.Fatalf("response = %#v", response)
	}
	closePayload := mustJSON(t, protocol.PtyTarget{ID: opened.ID})
	if _, err := sup.Handler().Handle(ctx, protocol.Message{Type: protocol.MessagePtyClose, WorkspaceID: "w", Payload: closePayload}); err != nil {
		t.Fatal(err)
	}
}

func TestPtySessionIsWorkspaceBound(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "other", Root: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(s)
	defer h.Close()
	response, err := h.Handle(ctx, protocol.Message{Type: protocol.MessagePtyOpen, WorkspaceID: "w", Payload: mustJSON(t, protocol.PtyOpenRequest{Rows: 24, Cols: 80})})
	if err != nil {
		t.Fatal(err)
	}
	var opened protocol.PtyOpened
	if err := json.Unmarshal(response.Payload, &opened); err != nil {
		t.Fatal(err)
	}
	readPayload := mustJSON(t, protocol.PtyTarget{ID: opened.ID})
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessagePtyRead, WorkspaceID: "other", Payload: readPayload}); err == nil {
		t.Fatal("cross-workspace read accepted")
	}
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessagePtyWrite, WorkspaceID: "other", Payload: mustJSON(t, protocol.PtyTarget{ID: opened.ID, Data: []byte("x")})}); err == nil {
		t.Fatal("cross-workspace write accepted")
	}
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessagePtyClose, WorkspaceID: "w", Payload: readPayload}); err != nil {
		t.Fatal(err)
	}
}
