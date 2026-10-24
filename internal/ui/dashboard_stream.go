package ui

import (
	"context"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/client"
	"termide/internal/domain"
)

// StreamDialer opens a dedicated framed-transport connection for push mode.
// Production wires client.DialSSHStream; tests inject net.Pipe pairs.
type StreamDialer func(context.Context) (io.ReadWriteCloser, error)

type streamOpenedResult struct {
	taskID string
	gen    uint64
	stream *client.Stream
	catch  []domain.EventEnvelope
	err    error
}

type streamEventResult struct {
	gen   uint64
	event domain.EventEnvelope
	err   error
}

// WithStreamDialer enables the live event stream with wait-loop fallback.
func (m Model) WithStreamDialer(dial StreamDialer) Model {
	m.dialStream = dial
	return m
}

func (m *Model) closeStream() {
	m.dashboard.streaming = false
	if m.dashboard.stream != nil {
		m.dashboard.stream.Close()
		m.dashboard.stream = nil
	}
}

// streamOpenTimeout bounds one dial+catch-up. A hung dial must not pin the
// open forever; the wait-loop fallback covers the gap. Tests override it.
var streamOpenTimeout = 15 * time.Second

// openStreamCommand dials a push connection for the current snapshot cursor.
// The open carries the stream generation so stale completions are discarded.
// On any failure the caller falls back to the wait loop.
func (m Model) openStreamCommand(taskID string, after uint64) tea.Cmd {
	dial := m.dialStream
	workspaceID := m.workspace.ID
	gen := m.dashboard.streamGen
	return func() tea.Msg {
		if dial == nil {
			return streamOpenedResult{taskID: taskID, gen: gen, err: context.Canceled}
		}
		ctx, cancel := context.WithTimeout(context.Background(), streamOpenTimeout)
		// No defer cancel: ownership transfers to the opened stream; error
		// paths cancel explicitly below.
		conn, err := dial(ctx)
		if err != nil {
			cancel()
			return streamOpenedResult{taskID: taskID, gen: gen, err: err}
		}
		stream, catchUp, err := client.OpenStream(ctx, conn, workspaceID, after)
		if err != nil {
			cancel()
			return streamOpenedResult{taskID: taskID, gen: gen, err: err}
		}
		stream.Detach()
		cancel()
		return streamOpenedResult{taskID: taskID, gen: gen, stream: stream, catch: catchUp}
	}
}

func (m Model) receiveStreamOpened(result streamOpenedResult) (tea.Model, tea.Cmd) {
	if result.gen != m.dashboard.streamGen {
		// Superseded by a newer stream: drop without touching live state.
		if result.stream != nil {
			result.stream.Close()
		}
		return m, nil
	}
	if result.err != nil || m.dashboard.snapshot == nil || m.dashboard.snapshot.Task.ID != result.taskID {
		if result.stream != nil {
			result.stream.Close()
		}
		// Fall back to the wait loop instead of error-spinning.
		if m.dashboard.snapshot != nil && !terminalTask(m.dashboard.snapshot.Task.Status) {
			if cmd := m.waitCommand(); cmd != nil {
				m.dashboard.waiting = true
				return m, cmd
			}
		}
		if result.err != nil && result.err != context.Canceled {
			m.Status = result.err.Error()
		}
		return m, nil
	}
	m.closeStream()
	m.dashboard.stream = result.stream
	m.dashboard.streaming = true
	previous := m.dashboard.snapshot.NextSequence
	for _, event := range result.catch {
		if event.WorkspaceID != m.workspace.ID || event.TaskID != result.taskID {
			continue
		}
		if err := event.Validate(); err != nil {
			continue
		}
		if event.Sequence <= previous {
			continue
		}
		previous = event.Sequence
		m.dashboard.snapshot.Events = append(m.dashboard.snapshot.Events, event)
		m.dashboard.logs = append(m.dashboard.logs, snapshotLogLines([]domain.EventEnvelope{event})...)
	}
	m.dashboard.snapshot.NextSequence = previous
	m.Status = "live stream connected"
	return m, m.streamNextCommand()
}

func (m Model) streamNextCommand() tea.Cmd {
	stream := m.dashboard.stream
	gen := m.dashboard.streamGen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), dashboardWaitBudget+5*time.Second)
		defer cancel()
		event, err := stream.Next(ctx)
		return streamEventResult{gen: gen, event: event, err: err}
	}
}

func (m Model) receiveStreamEvent(result streamEventResult) (tea.Model, tea.Cmd) {
	if result.gen != m.dashboard.streamGen {
		// Stale read from a superseded stream: ignore, keep the live one.
		return m, nil
	}
	if result.err != nil {
		m.closeStream()
		if m.dashboard.snapshot != nil && !terminalTask(m.dashboard.snapshot.Task.Status) {
			m.Status = "stream ended; waiting for updates"
			if cmd := m.waitCommand(); cmd != nil {
				m.dashboard.waiting = true
				return m, cmd
			}
		} else {
			m.Status = "stream ended"
		}
		return m, nil
	}
	if m.dashboard.snapshot != nil &&
		result.event.WorkspaceID == m.workspace.ID &&
		result.event.TaskID == m.dashboard.snapshot.Task.ID &&
		result.event.Validate() == nil &&
		result.event.Sequence > m.dashboard.snapshot.NextSequence {
		m.dashboard.snapshot.Events = append(m.dashboard.snapshot.Events, result.event)
		m.dashboard.snapshot.NextSequence = result.event.Sequence
		m.dashboard.logs = append(m.dashboard.logs, snapshotLogLines([]domain.EventEnvelope{result.event})...)
		m.Status = "live update received"
	}
	if m.dashboard.snapshot == nil || terminalTask(m.dashboard.snapshot.Task.Status) {
		m.closeStream()
		return m, nil
	}
	return m, m.streamNextCommand()
}
