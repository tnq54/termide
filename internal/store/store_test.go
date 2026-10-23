package store

import (
	"context"
	"testing"

	"termide/internal/domain"
)

func TestSQLiteStorePersistsTaskAndEvents(t *testing.T) {
	db := t.TempDir() + "/termide.db"
	ctx := context.Background()
	s, err := Open(db)
	if err != nil { t.Fatal(err) }
	defer s.Close()

	ws := domain.Workspace{ID: "w1", Name: "demo", Host: "vps", Root: "/srv/demo"}
	if err := s.CreateWorkspace(ctx, ws); err != nil { t.Fatal(err) }
	task := domain.Task{ID: "t1", WorkspaceID: ws.ID, Description: "run tests", Status: domain.TaskQueued}
	if err := s.CreateTask(ctx, task); err != nil { t.Fatal(err) }
	if err := s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w1", TaskID: "t1", AgentID: "a1", Type: domain.EventTaskUpdated, Payload: []byte(`{}`)}); err != nil { t.Fatal(err) }
	if err := s.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: "w1", TaskID: "t1", AgentID: "a1", Type: domain.EventOutput, Payload: []byte(`{"line":1}`)}); err != nil { t.Fatal(err) }

	got, err := s.GetTask(ctx, "t1")
	if err != nil { t.Fatal(err) }
	if got.Description != task.Description { t.Fatalf("description = %q", got.Description) }
	events, err := s.EventsAfter(ctx, "w1", 1)
	if err != nil { t.Fatal(err) }
	if len(events) != 1 || events[0].Sequence != 2 { t.Fatalf("events = %#v", events) }
}
