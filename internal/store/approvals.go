package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"termide/internal/domain"
)

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

func (s *Store) CreateApproval(ctx context.Context, request domain.ApprovalRequest) error {
	if request.ID == "" || request.TaskID == "" || request.AgentID == "" || request.Command == "" {
		return fmt.Errorf("approval identity and command are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT id FROM approvals WHERE task_id=? AND agent_id=? AND status=?`, request.TaskID, request.AgentID, string(domain.ApprovalPending)).Scan(&existing)
	if err == nil {
		return fmt.Errorf("agent already has a pending approval")
	}
	if err != nil && !isNoRows(err) {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO approvals(id,task_id,agent_id,command,reason,status) VALUES(?,?,?,?,?,?)`, request.ID, request.TaskID, request.AgentID, request.Command, request.Reason, string(domain.ApprovalPending)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetApproval(ctx context.Context, id string) (domain.ApprovalRequest, error) {
	var request domain.ApprovalRequest
	var status string
	err := s.db.QueryRowContext(ctx, `SELECT id,task_id,agent_id,command,reason,status FROM approvals WHERE id=?`, id).
		Scan(&request.ID, &request.TaskID, &request.AgentID, &request.Command, &request.Reason, &status)
	request.Status = domain.ApprovalStatus(status)
	return request, err
}

func (s *Store) ListPendingApprovals(ctx context.Context, taskID string) ([]domain.ApprovalRequest, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,task_id,agent_id,command,reason,status FROM approvals WHERE task_id=? AND status=? ORDER BY rowid`, taskID, string(domain.ApprovalPending))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ApprovalRequest
	for rows.Next() {
		var request domain.ApprovalRequest
		var status string
		if err := rows.Scan(&request.ID, &request.TaskID, &request.AgentID, &request.Command, &request.Reason, &status); err != nil {
			return nil, err
		}
		request.Status = domain.ApprovalStatus(status)
		out = append(out, request)
	}
	return out, rows.Err()
}

func (s *Store) ResolveApproval(ctx context.Context, id string, approved bool, reason string) error {
	status := string(domain.ApprovalDenied)
	if approved {
		status = string(domain.ApprovalApproved)
	}
	result, err := s.db.ExecContext(ctx, `UPDATE approvals SET status=?,resolved_reason=? WHERE id=? AND status=?`, status, reason, id, string(domain.ApprovalPending))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("approval is not pending")
	}
	return nil
}
