package store

import (
	"context"
	"fmt"
	"math"
	"termide/internal/domain"
)

const MaxSnapshotPayloadBytes = 2 * 1024 * 1024

func (s *Store) TaskPage(ctx context.Context, workspaceID string, before int64, limit int) (domain.TaskPage, error) {
	page := domain.TaskPage{Tasks: []domain.Task{}}
	if workspaceID == "" || before < 0 || limit < 1 || limit > 100 {
		return page, fmt.Errorf("invalid task page request")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT rowid,id,workspace_id,description,status FROM tasks WHERE workspace_id=? AND (?=0 OR rowid<?) ORDER BY rowid DESC LIMIT ?`, workspaceID, before, before, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var last int64
	for rows.Next() {
		if len(page.Tasks) == limit {
			page.Next = last
			break
		}
		var task domain.Task
		if err := rows.Scan(&last, &task.ID, &task.WorkspaceID, &task.Description, &task.Status); err != nil {
			return page, err
		}
		page.Tasks = append(page.Tasks, task)
	}
	return page, rows.Err()
}

func (s *Store) TaskSnapshot(ctx context.Context, workspaceID, taskID string, after uint64, limit int) (domain.TaskSnapshot, error) {
	snapshot := domain.TaskSnapshot{Agents: []domain.Agent{}, Events: []domain.EventEnvelope{}, NextSequence: after}
	if workspaceID == "" || taskID == "" || after > math.MaxInt64 || limit < 1 || limit > 100 {
		return snapshot, fmt.Errorf("invalid task snapshot request")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return snapshot, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `SELECT id,workspace_id,description,status FROM tasks WHERE id=? AND workspace_id=?`, taskID, workspaceID).Scan(&snapshot.Task.ID, &snapshot.Task.WorkspaceID, &snapshot.Task.Description, &snapshot.Task.Status)
	if err != nil {
		return snapshot, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,task_id,role,status,COALESCE(branch,''),COALESCE(worktree,''),COALESCE(summary,'') FROM agents WHERE task_id=? ORDER BY id LIMIT 101`, taskID)
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var a domain.Agent
		if err := rows.Scan(&a.ID, &a.TaskID, &a.Role, &a.Status, &a.Branch, &a.Worktree, &a.Summary); err != nil {
			rows.Close()
			return snapshot, err
		}
		snapshot.Agents = append(snapshot.Agents, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return snapshot, err
	}
	if len(snapshot.Agents) > 100 {
		return snapshot, fmt.Errorf("task exceeds 100-agent dashboard limit")
	}
	rows, err = tx.QueryContext(ctx, `SELECT workspace_id,task_id,agent_id,sequence,type,payload FROM events WHERE workspace_id=? AND task_id=? AND sequence>? ORDER BY sequence LIMIT ?`, workspaceID, taskID, after, limit+1)
	if err != nil {
		return snapshot, err
	}
	bytes := 0
	for rows.Next() {
		if len(snapshot.Events) == limit {
			snapshot.More = true
			break
		}
		var event domain.EventEnvelope
		if err := rows.Scan(&event.WorkspaceID, &event.TaskID, &event.AgentID, &event.Sequence, &event.Type, &event.Payload); err != nil {
			rows.Close()
			return snapshot, err
		}
		if len(event.Payload) > MaxSnapshotPayloadBytes {
			rows.Close()
			return snapshot, fmt.Errorf("event %d exceeds snapshot payload limit", event.Sequence)
		}
		if bytes+len(event.Payload) > MaxSnapshotPayloadBytes {
			snapshot.More = true
			break
		}
		snapshot.Events = append(snapshot.Events, event)
		snapshot.NextSequence = event.Sequence
		bytes += len(event.Payload)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return snapshot, err
	}
	return snapshot, tx.Commit()
}
