package client

import (
	"context"
	"net"
	"testing"
	"time"

	"termide/internal/domain"
	"termide/internal/protocol"
)

func TestRemoteSubscribeWaitSendsBudgetAndValidates(t *testing.T) {
	events := []domain.EventEnvelope{
		{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 2, Type: domain.EventOutput},
	}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	requested := make(chan protocol.Message, 1)
	go func() {
		request, err := protocol.Read(right)
		if err != nil {
			return
		}
		requested <- request
		response := protocol.Message{Type: protocol.MessageReplay, WorkspaceID: "w", RequestID: request.RequestID, Payload: mustJSON(t, events)}
		_ = protocol.Write(right, response)
	}()
	remote := NewRemote(left)
	defer remote.Close()
	result, err := remote.SubscribeWait(context.Background(), "w", 1, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].Sequence != 2 {
		t.Fatalf("result = %#v", result)
	}
	select {
	case request := <-requested:
		if request.Type != protocol.MessageSubscribe || request.AfterSequence != 1 || request.WaitMs != 2000 {
			t.Fatalf("request = %#v", request)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request not observed")
	}
}

func TestRemoteSubscribeWaitRejectsBadBudget(t *testing.T) {
	remote, done := replayRemote(t, protocol.Message{})
	defer func() { remote.Close(); <-done }()
	if _, err := remote.SubscribeWait(context.Background(), "", 0, time.Second); err == nil {
		t.Fatal("empty workspace accepted")
	}
	if _, err := remote.SubscribeWait(context.Background(), "w", 0, 0); err == nil {
		t.Fatal("non-positive wait accepted")
	}
}
