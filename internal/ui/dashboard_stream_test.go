package ui

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
	"termide/internal/protocol"
)

type fakeStreamServer struct {
	requests chan protocol.Message
	write    chan protocol.Message
}

func serveStreamTest(t *testing.T, conn io.ReadWriteCloser) *fakeStreamServer {
	t.Helper()
	srv := &fakeStreamServer{requests: make(chan protocol.Message, 4), write: make(chan protocol.Message, 8)}
	go func() {
		defer conn.Close()
		request, err := protocol.Read(conn)
		if err != nil {
			return
		}
		srv.requests <- request
		for message := range srv.write {
			if err := protocol.Write(conn, message); err != nil {
				return
			}
		}
	}()
	return srv
}

func streamModel(t *testing.T) (Model, <-chan *fakeStreamServer) {
	t.Helper()
	srvCh := make(chan *fakeStreamServer, 1)
	m := NewRemoteModel(dashboardReply(func(protocol.Message) (protocol.Message, error) {
		return protocol.Message{}, nil
	}), domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning}, NextSequence: 2}
	m = m.WithStreamDialer(func(context.Context) (io.ReadWriteCloser, error) {
		left, right := net.Pipe()
		srvCh <- serveStreamTest(t, right)
		return left, nil
	})
	return m, srvCh
}

func awaitServer(t *testing.T, srvCh <-chan *fakeStreamServer) *fakeStreamServer {
	t.Helper()
	select {
	case srv := <-srvCh:
		return srv
	case <-time.After(2 * time.Second):
		t.Fatal("dialer did not run")
		return nil
	}
}

func TestStreamOpensOnSnapshotAndAppliesCatchUp(t *testing.T) {
	m, srvCh := streamModel(t)
	next, cmd := m.receiveDashboard(dashboardResult{
		kind:     protocol.MessageTaskSnapshot,
		snapshot: domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning}, NextSequence: 2},
	})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("stream was not opened")
	}
	msgDone := make(chan tea.Msg, 1)
	go func() { msgDone <- cmd() }()
	srv := awaitServer(t, srvCh)
	select {
	case request := <-srv.requests:
		if request.Type != protocol.MessageSubscribeStream || request.AfterSequence != 2 {
			t.Fatalf("request = %#v", request)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stream request not sent")
	}
	events, _ := json.Marshal([]domain.EventEnvelope{{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 3, Type: domain.EventOutput, Payload: []byte(`{"Output":"live"}`)}})
	srv.write <- protocol.Message{Type: protocol.MessageReplay, WorkspaceID: "w", Payload: events}
	var msg tea.Msg
	select {
	case msg = <-msgDone:
	case <-time.After(2 * time.Second):
		t.Fatal("open command did not return")
	}
	opened, ok := msg.(streamOpenedResult)
	if !ok {
		t.Fatalf("msg = %#v", msg)
	}
	next, follow := m.receiveStreamOpened(opened)
	m = next.(Model)
	if opened.err != nil {
		t.Fatal(opened.err)
	}
	if follow == nil {
		t.Fatal("read loop not scheduled")
	}
	if m.dashboard.snapshot.NextSequence != 3 || len(m.dashboard.logs) != 1 {
		t.Fatalf("model = %#v", m.dashboard)
	}
}

func TestStreamPushesLiveEvents(t *testing.T) {
	m, _ := streamModel(t)
	m.dashboard.streaming = true
	next, cmd := m.receiveStreamEvent(streamEventResult{event: domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 5, Type: domain.EventOutput, Payload: []byte(`{"Output":"push"}`)}})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("read loop did not continue")
	}
	if len(m.dashboard.logs) != 1 || m.dashboard.snapshot.NextSequence != 5 {
		t.Fatalf("model = %#v", m.dashboard)
	}
}

func TestStreamDialErrorFallsBackToWait(t *testing.T) {
	m := NewRemoteModel(dashboardReply(func(protocol.Message) (protocol.Message, error) {
		return protocol.Message{}, nil
	}), domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning}, NextSequence: 0}
	m = m.WithStreamDialer(func(context.Context) (io.ReadWriteCloser, error) {
		return nil, errStreamBoom{}
	})
	next, cmd := m.receiveDashboard(dashboardResult{
		kind:     protocol.MessageTaskSnapshot,
		snapshot: domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning}, NextSequence: 0},
	})
	m = next.(Model)
	_ = cmd
	if m.dashboard.streaming {
		t.Fatal("stream marked active after dial failure")
	}
}

func TestStaleStreamEventIsIgnored(t *testing.T) {
	m, _ := streamModel(t)
	m.dashboard.streaming = true
	m.dashboard.streamGen = 2
	next, cmd := m.receiveStreamEvent(streamEventResult{gen: 1, event: domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 9, Type: domain.EventOutput, Payload: []byte(`{"Output":"stale"}`)}})
	m = next.(Model)
	if cmd != nil || len(m.dashboard.logs) != 0 {
		t.Fatal("stale stream event was applied")
	}
	if !m.dashboard.streaming {
		t.Fatal("live stream was killed by stale completion")
	}
}

func TestStreamOpenTimeoutFallsBackToWait(t *testing.T) {
	old := streamOpenTimeout
	streamOpenTimeout = 100 * time.Millisecond
	defer func() { streamOpenTimeout = old }()
	m := NewRemoteModel(dashboardReply(func(protocol.Message) (protocol.Message, error) {
		return protocol.Message{}, nil
	}), domain.Workspace{ID: "w"})
	m.ready = true
	m.Panel = PanelAgents
	m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning}, NextSequence: 0}
	m = m.WithStreamDialer(func(ctx context.Context) (io.ReadWriteCloser, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	next, cmd := m.receiveDashboard(dashboardResult{
		kind:     protocol.MessageTaskSnapshot,
		snapshot: domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskRunning}, NextSequence: 0},
	})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("open not scheduled")
	}
	next, follow := m.Update(cmd())
	m = next.(Model)
	if m.dashboard.streaming {
		t.Fatal("hung dial produced a stream")
	}
	_ = follow
}

func TestStreamStopsOnTerminalSnapshot(t *testing.T) {
	m, _ := streamModel(t)
	m.dashboard.streaming = true
	next, cmd := m.receiveDashboard(dashboardResult{
		kind:     protocol.MessageTaskSnapshot,
		snapshot: domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskCompleted}, NextSequence: 9},
	})
	m = next.(Model)
	if cmd != nil || m.dashboard.streaming {
		t.Fatal("stream continued for terminal task")
	}
}

type errStreamBoom struct{}

func (errStreamBoom) Error() string { return "dial failed" }
