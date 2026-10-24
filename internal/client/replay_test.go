package client

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
)

func TestRemoteSubscribeValidatesReplayIdentityAndSequence(t *testing.T) {
	events := []domain.EventEnvelope{
		{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 2, Type: domain.EventOutput},
		{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 4, Type: domain.EventOutput},
	}
	remote, done := replayRemote(t, protocol.Message{Type: protocol.MessageReplay, WorkspaceID: "w", Payload: mustJSON(t, events)})
	defer func() { remote.Close(); <-done }()

	got, err := remote.Subscribe(context.Background(), "w", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Sequence != 2 || got[1].Sequence != 4 {
		t.Fatalf("events = %#v", got)
	}
}

func TestRemoteSubscribeRejectsInvalidReplay(t *testing.T) {
	tests := []struct {
		name   string
		events []domain.EventEnvelope
	}{
		{"wrong workspace", []domain.EventEnvelope{{WorkspaceID: "other", TaskID: "t", AgentID: "a", Sequence: 2, Type: domain.EventOutput}}},
		{"not after cursor", []domain.EventEnvelope{{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 1, Type: domain.EventOutput}}},
		{"not increasing", []domain.EventEnvelope{{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 3, Type: domain.EventOutput}, {WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 3, Type: domain.EventOutput}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			remote, done := replayRemote(t, protocol.Message{Type: protocol.MessageReplay, WorkspaceID: "w", Payload: mustJSON(t, test.events)})
			defer func() { remote.Close(); <-done }()
			if _, err := remote.Subscribe(context.Background(), "w", 1); err == nil {
				t.Fatal("invalid replay accepted")
			}
		})
	}
}

func replayRemote(t *testing.T, response protocol.Message) (*Remote, <-chan error) {
	t.Helper()
	left, right := net.Pipe()
	done := make(chan error, 1)
	go func() {
		defer right.Close()
		request, err := protocol.Read(right)
		if err != nil {
			done <- err
			return
		}
		response.RequestID = request.RequestID
		done <- protocol.Write(right, response)
	}()
	return NewRemote(left), done
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
