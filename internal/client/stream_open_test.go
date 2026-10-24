package client

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
)

func TestStreamNextReturnsCatchUpThenPush(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	go func() {
		request, err := protocol.Read(right)
		if err != nil {
			return
		}
		if request.Type != protocol.MessageSubscribeStream {
			panic(request)
		}
		events, _ := json.Marshal([]domain.EventEnvelope{{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 2, Type: domain.EventOutput}})
		_ = protocol.Write(right, protocol.Message{Type: protocol.MessageReplay, WorkspaceID: "w", RequestID: request.RequestID, Payload: events})
		push := domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 3, Type: domain.EventOutput}
		_ = protocol.Write(right, protocol.Message{Type: protocol.MessageEvent, WorkspaceID: "w", RequestID: request.RequestID, TaskID: "t", Event: &push})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stream, catchUp, err := OpenStream(ctx, left, "w", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if len(catchUp) != 1 || catchUp[0].Sequence != 2 {
		t.Fatalf("catchUp = %#v", catchUp)
	}
	event, err := stream.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if event.Sequence != 3 {
		t.Fatalf("event = %#v", event)
	}
}
