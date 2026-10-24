package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
)

// dashboardWaitBudget bounds one live-wait cycle. Manual commands queue
// behind an in-flight wait on the serial transport, so the budget also caps
// that delay.
const dashboardWaitBudget = 10 * time.Second

type waitCaller interface {
	SubscribeWait(context.Context, string, uint64, time.Duration) ([]domain.EventEnvelope, error)
}

type dashboardWaitResult struct {
	after  uint64
	events []domain.EventEnvelope
	err    error
}

// waitCommand schedules one live-wait cycle from the current snapshot cursor.
// It never sets busy: scrolling and commands keep working while waiting.
func (m Model) waitCommand() tea.Cmd {
	if m.remote == nil || !m.ready || m.dashboard.snapshot == nil || m.dashboard.waiting {
		return nil
	}
	if terminalTask(m.dashboard.snapshot.Task.Status) {
		return nil
	}
	waiter, ok := m.remote.(waitCaller)
	if !ok {
		return nil
	}
	after := m.dashboard.snapshot.NextSequence
	workspaceID := m.workspace.ID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), dashboardWaitBudget+5*time.Second)
		defer cancel()
		events, err := waiter.SubscribeWait(ctx, workspaceID, after, dashboardWaitBudget)
		return dashboardWaitResult{after: after, events: events, err: err}
	}
}

func terminalTask(status domain.TaskStatus) bool {
	return status == domain.TaskCompleted || status == domain.TaskFailed || status == domain.TaskCancelled
}

func (m Model) receiveDashboardWait(result dashboardWaitResult) (tea.Model, tea.Cmd) {
	m.dashboard.waiting = false
	if result.err != nil {
		m.Status = result.err.Error()
		return m, nil
	}
	if m.dashboard.snapshot == nil {
		return m, nil
	}
	var kept []domain.EventEnvelope
	for _, event := range result.events {
		if event.WorkspaceID != m.workspace.ID || event.TaskID != m.dashboard.snapshot.Task.ID {
			continue
		}
		if err := event.Validate(); err != nil {
			continue
		}
		if event.Sequence <= m.dashboard.snapshot.NextSequence {
			continue
		}
		kept = append(kept, event)
		if event.Sequence > m.dashboard.snapshot.NextSequence {
			m.dashboard.snapshot.NextSequence = event.Sequence
		}
	}
	if len(kept) > 0 {
		m.dashboard.snapshot.Events = append(m.dashboard.snapshot.Events, kept...)
		m.dashboard.logs = append(m.dashboard.logs, snapshotLogLines(kept)...)
		m.Status = "live update received"
	}
	if terminalTask(m.dashboard.snapshot.Task.Status) {
		return m, nil
	}
	if cmd := m.waitCommand(); cmd != nil {
		m.dashboard.waiting = true
		return m, cmd
	}
	return m, nil
}
