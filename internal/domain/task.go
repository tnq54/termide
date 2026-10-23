package domain

import (
	"fmt"
	"time"
)

type TaskStatus string

const (
	TaskQueued           TaskStatus = "queued"
	TaskPlanning         TaskStatus = "planning"
	TaskRunning          TaskStatus = "running"
	TaskWaitingApproval  TaskStatus = "waiting_approval"
	TaskBlocked          TaskStatus = "blocked"
	TaskCompleted        TaskStatus = "completed"
	TaskFailed           TaskStatus = "failed"
	TaskCancelled        TaskStatus = "cancelled"
)

type Task struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (t *Task) Transition(next TaskStatus) error {
	if t.Status == next { return nil }
	valid := map[TaskStatus]map[TaskStatus]bool{
		TaskQueued: {TaskPlanning: true, TaskRunning: true, TaskCancelled: true},
		TaskPlanning: {TaskRunning: true, TaskFailed: true, TaskCancelled: true},
		TaskRunning: {TaskWaitingApproval: true, TaskBlocked: true, TaskCompleted: true, TaskFailed: true, TaskCancelled: true},
		TaskWaitingApproval: {TaskRunning: true, TaskBlocked: true, TaskCancelled: true},
		TaskBlocked: {TaskRunning: true, TaskFailed: true, TaskCancelled: true},
	}
	if !valid[t.Status][next] { return fmt.Errorf("invalid task transition %q -> %q", t.Status, next) }
	t.Status = next
	t.UpdatedAt = time.Now().UTC()
	return nil
}
