package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"termide/internal/domain"
)

type TaskRun struct {
	RunID           string
	TaskID          string
	PlanJSON        []byte
	Owner           string
	LeaseUntil      time.Time
	Attempt         int
	CancelRequested bool
	Status          string
	Error           string
}

func (s *Store) ClaimTaskRun(ctx context.Context, taskID string, planJSON []byte, owner string, leaseUntil time.Time) (TaskRun, error) {
	if taskID == "" || owner == "" || len(planJSON) == 0 {
		return TaskRun{}, fmt.Errorf("task run identity and plan are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TaskRun{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE tasks SET status=? WHERE id=? AND status=?`, domain.TaskRunning, taskID, domain.TaskQueued)
	if err != nil {
		return TaskRun{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return TaskRun{}, err
	}
	if count != 1 {
		return TaskRun{}, fmt.Errorf("task is not queued")
	}
	run := TaskRun{RunID: uuid.NewString(), TaskID: taskID, PlanJSON: append([]byte(nil), planJSON...), Owner: owner, LeaseUntil: leaseUntil, Attempt: 1, Status: string(domain.TaskRunning)}
	_, err = tx.ExecContext(ctx, `INSERT INTO task_runs(run_id,task_id,plan_json,owner,lease_until,attempt,status) VALUES(?,?,?,?,?,?,?)`, run.RunID, run.TaskID, run.PlanJSON, run.Owner, run.LeaseUntil.UnixNano(), run.Attempt, run.Status)
	if err != nil {
		return TaskRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return TaskRun{}, err
	}
	return run, nil
}

func (s *Store) GetTaskRun(ctx context.Context, taskID string) (TaskRun, error) {
	var run TaskRun
	var lease int64
	var cancel int
	err := s.db.QueryRowContext(ctx, `SELECT run_id,task_id,plan_json,owner,lease_until,attempt,cancel_requested,status,error FROM task_runs WHERE task_id=?`, taskID).
		Scan(&run.RunID, &run.TaskID, &run.PlanJSON, &run.Owner, &lease, &run.Attempt, &cancel, &run.Status, &run.Error)
	run.LeaseUntil = time.Unix(0, lease)
	run.CancelRequested = cancel != 0
	return run, err
}

func (s *Store) RequestTaskCancel(ctx context.Context, taskID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE task_runs SET cancel_requested=1 WHERE task_id=? AND status=?`, taskID, string(domain.TaskRunning))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("task run is not active")
	}
	return nil
}

func (s *Store) RecoverExpiredRuns(ctx context.Context, owner string, now time.Time) ([]TaskRun, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT run_id,task_id,plan_json,owner,lease_until,attempt,cancel_requested,status,error FROM task_runs WHERE status=? AND lease_until<?`, string(domain.TaskRunning), now.UnixNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []TaskRun
	for rows.Next() {
		var run TaskRun
		var lease int64
		var cancel int
		if err := rows.Scan(&run.RunID, &run.TaskID, &run.PlanJSON, &run.Owner, &lease, &run.Attempt, &cancel, &run.Status, &run.Error); err != nil {
			return nil, err
		}
		run.LeaseUntil = time.Unix(0, lease)
		run.CancelRequested = cancel != 0
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range runs {
		result, err := s.db.ExecContext(ctx, `UPDATE task_runs SET owner=?,lease_until=?,attempt=?,cancel_requested=0 WHERE run_id=? AND status=? AND lease_until<?`, owner, now.Add(time.Minute).UnixNano(), runs[i].Attempt+1, runs[i].RunID, string(domain.TaskRunning), now.UnixNano())
		if err != nil {
			return nil, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if n == 1 {
			if _, err := s.db.ExecContext(ctx, `UPDATE tasks SET status=? WHERE id=? AND status=?`, domain.TaskQueued, runs[i].TaskID, domain.TaskRunning); err != nil {
				return nil, err
			}
			runs[i].Owner = owner
			runs[i].LeaseUntil = now.Add(time.Minute)
			runs[i].Attempt++
			runs[i].CancelRequested = false
		} else {
			runs[i].RunID = ""
		}
	}
	out := runs[:0]
	for _, run := range runs {
		if run.RunID != "" {
			out = append(out, run)
		}
	}
	return out, nil
}

func (s *Store) FinishTaskRun(ctx context.Context, runID string, status domain.TaskStatus, errText string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE task_runs SET status=?,error=? WHERE run_id=? AND status=?`, string(status), errText, runID, string(domain.TaskRunning))
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return fmt.Errorf("task run is not active")
	}
	return nil
}
