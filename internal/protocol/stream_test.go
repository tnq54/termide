package protocol

import (
	"bytes"
	"testing"
)

func TestReadConsecutiveFrames(t *testing.T) {
	var stream bytes.Buffer
	for _, id := range []string{"one", "two"} {
		if err := Write(&stream, Message{Type: MessageHello, RequestID: id}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"one", "two"} {
		message, err := Read(&stream)
		if err != nil {
			t.Fatal(err)
		}
		if message.RequestID != id {
			t.Fatal(message)
		}
	}
}

func TestReadRejectsInvalidTerminator(t *testing.T) {
	var stream bytes.Buffer
	if err := Write(&stream, Message{Type: MessageHello}); err != nil {
		t.Fatal(err)
	}
	data := stream.Bytes()
	data[len(data)-1] = 'x'
	if _, err := Read(bytes.NewReader(data)); err == nil {
		t.Fatal("invalid delimiter accepted")
	}
}
