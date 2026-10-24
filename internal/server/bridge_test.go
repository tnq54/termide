package server

import (
	"context"
	"net"
	"testing"

	"termide/internal/protocol"
)

func TestBridgeForwardsStartAndCancelToControl(t *testing.T) {
	controlClient, controlServer := net.Pipe()
	defer controlClient.Close()
	defer controlServer.Close()
	bridge := NewBridge(controlClient)
	client, bridgeServer := net.Pipe()
	defer client.Close()
	defer bridgeServer.Close()
	done := make(chan error, 1)
	go func() { done <- bridge.Serve(context.Background(), bridgeServer) }()
	if err := protocol.Write(client, protocol.Message{Type: protocol.MessageCancel, RequestID: "1", WorkspaceID: "w", TaskID: "t"}); err != nil {
		t.Fatal(err)
	}
	request, err := protocol.Read(controlServer)
	if err != nil {
		t.Fatal(err)
	}
	if request.Type != protocol.MessageCancel || request.RequestID != "1" {
		t.Fatal(request)
	}
	if err := protocol.Write(controlServer, protocol.Message{Type: protocol.MessageTaskCancelRequested, RequestID: "1", WorkspaceID: "w", TaskID: "t"}); err != nil {
		t.Fatal(err)
	}
	response, err := protocol.Read(client)
	if err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageTaskCancelRequested || response.RequestID != "1" {
		t.Fatal(response)
	}
	client.Close()
	<-done
}
