package store

import (
	"context"
	"fmt"
	"termide/internal/domain"
	"testing"
)

func TestTaskPagesAreWorkspaceScopedAndStable(t *testing.T) {
	s, err := Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	for _, id := range []string{"w", "other"} {
		if err := s.CreateWorkspace(ctx, domain.Workspace{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 5; i++ {
		if err := s.CreateTask(ctx, domain.Task{ID: fmt.Sprintf("t%d", i), WorkspaceID: "w", Status: domain.TaskQueued}); err != nil {
			t.Fatal(err)
		}
	}
	s.CreateTask(ctx, domain.Task{ID: "hidden", WorkspaceID: "other", Status: domain.TaskQueued})
	page, err := s.TaskPage(ctx, "w", 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Tasks) != 2 || page.Tasks[0].ID != "t4" || page.Next == 0 {
		t.Fatal(page)
	}
	s.CreateTask(ctx, domain.Task{ID: "new", WorkspaceID: "w", Status: domain.TaskQueued})
	next, err := s.TaskPage(ctx, "w", page.Next, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Tasks) != 2 || next.Tasks[0].ID != "t2" || next.Tasks[1].ID != "t1" {
		t.Fatal(next)
	}
	last, err := s.TaskPage(ctx, "w", next.Next, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Tasks) != 1 || last.Tasks[0].ID != "t0" || last.Next != 0 {
		t.Fatal(last)
	}
}

func TestSnapshotFiltersEventsAndRejectsWrongWorkspace(t *testing.T) {
	s, err := Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	for _, id := range []string{"w", "other"} {
		s.CreateWorkspace(ctx, domain.Workspace{ID: id})
	}
	for _, id := range []string{"t", "unrelated"} {
		s.CreateTask(ctx, domain.Task{ID: id, WorkspaceID: "w", Status: domain.TaskRunning})
	}
	s.UpsertAgent(ctx, domain.Agent{ID: "a", TaskID: "t", Role: "coder", Status: domain.AgentRunning})
	for i := 0; i < 4; i++ {
		s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w", TaskID: "t", AgentID: "a", Type: domain.EventOutput, Payload: []byte(`{"Output":"hello"}`)})
		s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w", TaskID: "unrelated", AgentID: "x", Type: domain.EventOutput})
	}
	snapshot, err := s.TaskSnapshot(ctx, "w", "t", 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Task.ID != "t" || len(snapshot.Agents) != 1 || len(snapshot.Events) != 2 || !snapshot.More {
		t.Fatal(snapshot)
	}
	next, err := s.TaskSnapshot(ctx, "w", "t", snapshot.NextSequence, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Events) != 2 || next.More || next.Events[0].Sequence <= snapshot.NextSequence {
		t.Fatal(next)
	}
	for _, event := range next.Events {
		if event.TaskID != "t" {
			t.Fatal("cross-task event leak")
		}
	}
	if _, err := s.TaskSnapshot(ctx, "other", "t", 0, 2); err == nil {
		t.Fatal("cross-workspace snapshot accepted")
	}
}
