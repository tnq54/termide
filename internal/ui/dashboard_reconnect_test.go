package ui

import (
	"context"
	"testing"

	"termide/internal/domain"
	"termide/internal/protocol"
)

func TestWorkspaceReconnectSchedulesDashboardReplay(t *testing.T) {
	remote := &replayRemote{events: []domain.EventEnvelope{{WorkspaceID: "w", TaskID: "t", AgentID: "a", Sequence: 3, Type: domain.EventOutput}}}
	m := NewRemoteModel(remote, domain.Workspace{ID: "w"})
	m.dashboard.snapshot = &domain.TaskSnapshot{Task: domain.Task{ID: "t", WorkspaceID: "w"}, NextSequence: 2}

	next, cmd := m.Update(remoteResult{})
	m = next.(Model)
	if cmd == nil || m.Status != "replaying task history" {
		t.Fatal("reconnect did not schedule replay")
	}
	result := cmd().(dashboardReplayResult)
	if result.err != nil || remote.calls != 1 {
		t.Fatal(result.err)
	}
}

var _ remoteCaller = (*replayRemote)(nil)
var _ interface {
	Subscribe(context.Context, string, uint64) ([]domain.EventEnvelope, error)
} = (*replayRemote)(nil)
var _ = protocol.Message{}
