package store

import (
	"context"
	"testing"
	"time"

	"termide/internal/domain"
)

func TestTaskRunClaimRejectsDuplicateAndPersistsCancel(t *testing.T) {
	ctx := context.Background()
	s, err := Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	lease := time.Now().Add(time.Minute)
	run, err := s.ClaimTaskRun(ctx, "t", []byte(`[{"id":"a"}]`), "owner-1", lease)
	if err != nil {
		t.Fatal(err)
	}
	if run.TaskID != "t" || run.Owner != "owner-1" || run.Attempt != 1 {
		t.Fatal(run)
	}
	if _, err := s.ClaimTaskRun(ctx, "t", []byte(`[]`), "owner-2", lease); err == nil {
		t.Fatal("duplicate claim accepted")
	}
	if err := s.RequestTaskCancel(ctx, "t"); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTaskRun(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if !got.CancelRequested {
		t.Fatal(got)
	}
}

func TestRecoverExpiredRunsRequeuesTaskOnce(t *testing.T) {
	ctx := context.Background()
	s, err := Open(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.CreateWorkspace(ctx, domain.Workspace{ID: "w"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTask(ctx, domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskQueued}); err != nil {
		t.Fatal(err)
	}
	run, err := s.ClaimTaskRun(ctx, "t", []byte(`[]`), "old", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := s.RecoverExpiredRuns(ctx, "new", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 || recovered[0].RunID != run.RunID || recovered[0].Attempt != 2 {
		t.Fatal(recovered)
	}
	task, err := s.GetTask(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.TaskQueued {
		t.Fatal(task)
	}
	again, err := s.RecoverExpiredRuns(ctx, "new", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatal(again)
	}
}
