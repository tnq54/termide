package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
	"termide/internal/store"
)

func ptyHandlerFixture(t *testing.T) (*Handler, context.Context) {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w", Root: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	return NewHandler(s), ctx
}

func ptyCall(t *testing.T, h *Handler, ctx context.Context, msgType protocol.MessageType, payload any) protocol.Message {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	response, err := h.Handle(ctx, protocol.Message{Type: msgType, WorkspaceID: "w", Payload: raw})
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func TestPtyOpenWriteReadClose(t *testing.T) {
	h, ctx := ptyHandlerFixture(t)
	opened, err := func() (protocol.PtyOpened, error) {
		var out protocol.PtyOpened
		response := ptyCall(t, h, ctx, protocol.MessagePtyOpen, protocol.PtyOpenRequest{Rows: 24, Cols: 80})
		if response.Type != protocol.MessagePtyOpened {
			t.Fatalf("response = %#v", response)
		}
		err := json.Unmarshal(response.Payload, &out)
		return out, err
	}()
	if err != nil || opened.ID == "" {
		t.Fatal(err, opened)
	}
	ptyCall(t, h, ctx, protocol.MessagePtyWrite, protocol.PtyTarget{ID: opened.ID, Data: []byte("echo pty-roundtrip\n")})
	deadline := time.Now().Add(5 * time.Second)
	var output protocol.PtyOutput
	for {
		response := ptyCall(t, h, ctx, protocol.MessagePtyRead, protocol.PtyTarget{ID: opened.ID, After: output.Next})
		output = protocol.PtyOutput{}
		if err := json.Unmarshal(response.Payload, &output); err != nil {
			t.Fatal(err)
		}
		if response.Type != protocol.MessagePtyOutput {
			t.Fatalf("response = %#v", response)
		}
		if strings.Contains(string(output.Data), "pty-roundtrip") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no roundtrip: %q", output.Data)
		}
		time.Sleep(50 * time.Millisecond)
	}
	ptyCall(t, h, ctx, protocol.MessagePtyResize, protocol.PtyTarget{ID: opened.ID, Rows: 30, Cols: 100})
	response := ptyCall(t, h, ctx, protocol.MessagePtyClose, protocol.PtyTarget{ID: opened.ID})
	if response.Type != protocol.MessagePtyOk {
		t.Fatalf("response = %#v", response)
	}
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessagePtyRead, WorkspaceID: "w", Payload: mustJSON(t, protocol.PtyTarget{ID: opened.ID})}); err == nil {
		t.Fatal("closed session still readable")
	}
}

func TestPtyRejectsEmptySessionID(t *testing.T) {
	h, ctx := ptyHandlerFixture(t)
	for _, msgType := range []protocol.MessageType{protocol.MessagePtyWrite, protocol.MessagePtyRead, protocol.MessagePtyResize, protocol.MessagePtyClose} {
		if _, err := h.Handle(ctx, protocol.Message{Type: msgType, WorkspaceID: "w", Payload: mustJSON(t, protocol.PtyTarget{})}); err == nil {
			t.Fatalf("%s accepted empty session ID", msgType)
		}
	}
}

func TestPtyRejectsForeignWorkspace(t *testing.T) {
	h, ctx := ptyHandlerFixture(t)
	raw, _ := json.Marshal(protocol.PtyOpenRequest{Rows: 24, Cols: 80})
	if _, err := h.Handle(ctx, protocol.Message{Type: protocol.MessagePtyOpen, WorkspaceID: "missing", Payload: raw}); err == nil {
		t.Fatal("unknown workspace accepted")
	}
}
