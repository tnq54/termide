package store

import (
	"context"
	"fmt"
	"termide/internal/domain"
)

func (s *Store) UpdateAgentWorkspace(ctx context.Context, taskID, id, branch, path string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE agents SET branch=?,worktree=? WHERE task_id=? AND id=?`, branch, path, taskID, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("agent not found in task")
	}
	return nil
}

// BeginTask atomically claims a queued task and prepares its agents.
func (s *Store) BeginTask(ctx context.Context, id string, agents []domain.Agent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE tasks SET status=? WHERE id=? AND status=?`, domain.TaskRunning, id, domain.TaskQueued)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("task is not queued")
	}
	for _, a := range agents {
		result, err = tx.ExecContext(ctx, `INSERT INTO agents(id,task_id,role,status,branch,worktree,summary) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status WHERE agents.task_id=excluded.task_id`, a.ID, id, a.Role, domain.AgentQueued, a.Branch, a.Worktree, a.Summary)
		if err != nil {
			return err
		}
		n, err = result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("agent belongs to another task")
		}
	}
	return tx.Commit()
}

func (s *Store) UpdateAgentStatus(ctx context.Context, taskID, id string, status domain.AgentStatus) error {
	result, err := s.db.ExecContext(ctx, `UPDATE agents SET status=? WHERE task_id=? AND id=?`, status, taskID, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("agent not found in task")
	}
	return nil
}

func (s *Store) CancelQueuedAgents(ctx context.Context, taskID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE agents SET status=? WHERE task_id=? AND status=?`, domain.AgentCancelled, taskID, domain.AgentQueued)
	return err
}
