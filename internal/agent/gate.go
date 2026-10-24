package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"termide/internal/domain"
	"termide/internal/security"
	"termide/internal/store"
)

// GateRunner enforces spec-level approval before delegating. Safe commands
// run immediately, blocked commands fail without an approval record, and
// commands requiring approval pause the agent until the durable approval is
// resolved. All gate state lives in the store so decisions survive reconnect.
type GateRunner struct {
	store         *store.Store
	delegate      Runner
	taskID        string
	workspaceRoot string
	poll          time.Duration
}

func NewGateRunner(s *store.Store, delegate Runner, taskID, workspaceRoot string, poll time.Duration) *GateRunner {
	if poll <= 0 {
		poll = 500 * time.Millisecond
	}
	return &GateRunner{store: s, delegate: delegate, taskID: taskID, workspaceRoot: workspaceRoot, poll: poll}
}

func (g *GateRunner) Run(ctx context.Context, spec Spec, emit func(Event)) error {
	switch security.Classify(spec.Command, g.workspaceRoot) {
	case security.Safe:
		return g.delegate.Run(ctx, spec, emit)
	case security.Blocked:
		return fmt.Errorf("command is blocked by policy")
	case security.ApprovalRequired:
		return g.runApproved(ctx, spec, emit)
	default:
		return fmt.Errorf("unknown classification")
	}
}

func (g *GateRunner) runApproved(ctx context.Context, spec Spec, emit func(Event)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	request := domain.ApprovalRequest{
		ID:         uuid.NewString(),
		TaskID:     g.taskID,
		AgentID:    spec.ID,
		Command:    spec.Command,
		Reason:     "command requires approval",
		WorkingDir: spec.WorkingDir,
		Status:     domain.ApprovalPending,
	}
	if err := g.store.CreateApproval(ctx, request); err != nil {
		return err
	}
	emit(Event{AgentID: spec.ID, Kind: string(domain.EventApproval), Output: spec.Command})
	if err := g.store.UpdateAgentStatus(ctx, g.taskID, spec.ID, domain.AgentWaitingApproval); err != nil {
		return err
	}
	ticker := time.NewTicker(g.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			approval, err := g.store.GetApproval(ctx, request.ID)
			if err != nil {
				return err
			}
			switch approval.Status {
			case domain.ApprovalApproved:
				if err := g.store.UpdateAgentStatus(ctx, g.taskID, spec.ID, domain.AgentRunning); err != nil {
					return err
				}
				return g.delegate.Run(ctx, spec, emit)
			case domain.ApprovalDenied:
				return fmt.Errorf("command denied by approver")
			case domain.ApprovalPending:
			default:
				return fmt.Errorf("unknown approval status %q", approval.Status)
			}
		}
	}
}
