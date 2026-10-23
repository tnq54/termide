package protocol

import (
	"bytes"
	"testing"

	"termide/internal/domain"
)

func TestCodecRoundTripsMessage(t *testing.T) {
	var buf bytes.Buffer
	want := Message{Type: MessageSubscribe, RequestID: "r1", WorkspaceID: "w1", AfterSequence: 4}
	if err := Write(&buf, want); err != nil { t.Fatal(err) }
	got, err := Read(&buf)
	if err != nil { t.Fatal(err) }
	if got.Type != want.Type || got.RequestID != want.RequestID || got.WorkspaceID != want.WorkspaceID || got.AfterSequence != want.AfterSequence { t.Fatalf("got %#v, want %#v", got, want) }
}

func TestEventMessageRejectsInvalidEvent(t *testing.T) {
	msg := Message{Type: MessageEvent, Event: &domain.EventEnvelope{WorkspaceID: "w"}}
	if err := msg.Validate(); err == nil { t.Fatal("expected invalid event") }
}
