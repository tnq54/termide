package server

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

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

func TestBridgeUsesSplitStdioStreams(t *testing.T) {
	controlClient, controlServer := net.Pipe()
	defer controlClient.Close()
	defer controlServer.Close()
	go func() {
		request, err := protocol.Read(controlServer)
		if err != nil {
			return
		}
		_ = protocol.Write(controlServer, protocol.Message{Type: protocol.MessageTaskCancelRequested, RequestID: request.RequestID, WorkspaceID: request.WorkspaceID, TaskID: request.TaskID})
	}()
	sessionReader, sessionWriter := io.Pipe()
	sessionOutReader, sessionOutWriter := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- NewBridge(controlClient).Serve(context.Background(), Stdio(sessionReader, sessionOutWriter))
	}()
	if err := protocol.Write(sessionWriter, protocol.Message{Type: protocol.MessageCancel, RequestID: "7", WorkspaceID: "w", TaskID: "t"}); err != nil {
		t.Fatal(err)
	}
	type readResult struct {
		response protocol.Message
		err      error
	}
	readDone := make(chan readResult, 1)
	go func() {
		response, err := protocol.Read(sessionOutReader)
		readDone <- readResult{response: response, err: err}
	}()
	var response protocol.Message
	select {
	case result := <-readDone:
		if result.err != nil {
			t.Fatal(result.err)
		}
		response = result.response
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not forward the response")
	}
	sessionWriter.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not exit")
	}
	if response.Type != protocol.MessageTaskCancelRequested || response.RequestID != "7" {
		t.Fatal(response)
	}
}
