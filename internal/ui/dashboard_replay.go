package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/domain"
)

type replayCaller interface {
	Subscribe(context.Context, string, uint64) ([]domain.EventEnvelope, error)
}

type dashboardReplayResult struct {
	events []domain.EventEnvelope
	err    error
}

func (m Model) dashboardReplayCommand() tea.Cmd {
	return func() tea.Msg {
		caller, ok := m.remote.(replayCaller)
		if !ok {
			return dashboardReplayResult{err: fmt.Errorf("remote does not support replay")}
		}
		if m.dashboard.snapshot == nil {
			return dashboardReplayResult{err: fmt.Errorf("task detail is not loaded")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		events, err := caller.Subscribe(ctx, m.workspace.ID, m.dashboard.snapshot.NextSequence)
		return dashboardReplayResult{events: events, err: err}
	}
}

func (m Model) receiveDashboardReplay(result dashboardReplayResult) (tea.Model, tea.Cmd) {
	if result.err != nil {
		m.Status = result.err.Error()
		return m, nil
	}
	if m.dashboard.snapshot == nil {
		m.Status = "task detail is not loaded"
		return m, nil
	}
	previous := m.dashboard.snapshot.NextSequence
	for _, event := range result.events {
		if event.WorkspaceID != m.workspace.ID || event.TaskID != m.dashboard.snapshot.Task.ID || event.Sequence <= previous {
			m.Status = "invalid replay event"
			return m, nil
		}
		if err := event.Validate(); err != nil {
			m.Status = err.Error()
			return m, nil
		}
		previous = event.Sequence
	}
	m.dashboard.snapshot.Events = append(m.dashboard.snapshot.Events, result.events...)
	m.dashboard.snapshot.NextSequence = previous
	m.dashboard.logs = append(m.dashboard.logs, snapshotLogLines(result.events)...)
	m.Status = "task history replayed"
	return m, nil
}
