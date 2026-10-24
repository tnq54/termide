package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"termide/internal/agent"
	"termide/internal/domain"
	"termide/internal/store"
)

const (
	supervisorLeaseTTL      = time.Minute
	supervisorRenewInterval = 20 * time.Second
	gatePollInterval        = 500 * time.Millisecond
)

type Supervisor struct {
	store   *store.Store
	runner  agent.Runner
	owner   string
	pty     *PTYRegistry
	lsp     *LSPManager
	lspBin  string
	lspArgs []string
	mu      sync.Mutex
	jobs    map[string]context.CancelFunc
	wg      sync.WaitGroup
	closed  bool
	errs    []error
}

func NewSupervisor(s *store.Store, runner agent.Runner, owner string) *Supervisor {
	return &Supervisor{store: s, runner: runner, owner: owner, jobs: make(map[string]context.CancelFunc), pty: NewPTYRegistry()}
}

// PTY returns the daemon-scoped terminal registry shared by every durable
// handler, so sessions survive reconnects.
func (s *Supervisor) PTY() *PTYRegistry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pty == nil {
		s.pty = NewPTYRegistry()
	}
	return s.pty
}

// Handler serves the full workspace protocol with durable start/cancel.
func (s *Supervisor) Handler() *Handler {
	return &Handler{store: s.store, runtime: agent.NewTaskRuntime(s.store), supervisor: s, pty: s.PTY(), lsp: s.LSP()}
}

// SetLSP configures the Python language server command used for hover.
func (s *Supervisor) SetLSP(binary string, args []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lspBin = binary
	s.lspArgs = append([]string(nil), args...)
	s.lsp = nil
}

// LSP returns the daemon-scoped language server manager shared by durable
// handlers. Nil binary means hover is unavailable until configured.
func (s *Supervisor) LSP() *LSPManager {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lsp == nil {
		s.lsp = NewLSPManager(s.lspBin, s.lspArgs)
	}
	return s.lsp
}

func (s *Supervisor) Start(ctx context.Context, workspaceID, taskID string, specs []agent.Spec) (store.TaskRun, error) {
	if workspaceID == "" || taskID == "" || s.runner == nil || len(specs) == 0 {
		return store.TaskRun{}, fmt.Errorf("workspace, task and nonempty runner plan are required")
	}
	task, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return store.TaskRun{}, err
	}
	if task.WorkspaceID != workspaceID {
		return store.TaskRun{}, fmt.Errorf("task does not belong to workspace")
	}
	seen := make(map[string]bool, len(specs))
	for i := range specs {
		if specs[i].ID == "" || seen[specs[i].ID] {
			return store.TaskRun{}, fmt.Errorf("agent IDs must be nonempty and unique")
		}
		seen[specs[i].ID] = true
		if owner, err := s.store.AgentTask(ctx, specs[i].ID); err == nil && owner != taskID {
			return store.TaskRun{}, fmt.Errorf("agent ID %q already belongs to another task", specs[i].ID)
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return store.TaskRun{}, err
		}
		if specs[i].Command == "" {
			specs[i].Command = task.Description
		}
	}
	plan, err := json.Marshal(specs)
	if err != nil {
		return store.TaskRun{}, err
	}
	workspace, err := s.store.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return store.TaskRun{}, err
	}
	run, err := s.store.ClaimTaskRun(ctx, taskID, plan, s.owner, time.Now().Add(supervisorLeaseTTL))
	if err != nil {
		return store.TaskRun{}, err
	}
	for _, spec := range specs {
		if err := s.store.UpsertAgent(ctx, domain.Agent{ID: spec.ID, TaskID: taskID, Role: spec.Role, Status: domain.AgentQueued}); err != nil {
			finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = s.store.CancelQueuedAgents(finalCtx, taskID)
			_ = s.store.UpdateTaskStatus(finalCtx, taskID, domain.TaskFailed)
			_ = s.store.FinishTaskRun(finalCtx, run.RunID, domain.TaskFailed, err.Error())
			cancel()
			return store.TaskRun{}, err
		}
	}
	s.launch(run, specs, agent.NewGateRunner(s.store, s.runner, taskID, workspace.Root, gatePollInterval))
	return run, nil
}

func (s *Supervisor) launch(run store.TaskRun, specs []agent.Spec, runner agent.Runner) {
	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		cancel()
		return
	}
	s.jobs[run.TaskID] = cancel
	s.wg.Add(1)
	s.mu.Unlock()
	go s.execute(runCtx, run, specs, runner)
}

func (s *Supervisor) execute(ctx context.Context, run store.TaskRun, specs []agent.Spec, runner agent.Runner) {
	defer s.wg.Done()
	defer func() { s.mu.Lock(); delete(s.jobs, run.TaskID); s.mu.Unlock() }()
	renewStop := make(chan struct{})
	go s.renewLoop(ctx, run, renewStop)
	defer close(renewStop)
	runErr := agent.NewScheduler(&durablePersistedRunner{store: s.store, runner: runner, taskID: run.TaskID}, 4).Run(ctx, specs, func(agent.Event) {})
	s.finish(ctx, run, runErr)
}

func (s *Supervisor) renewLoop(ctx context.Context, run store.TaskRun, stop <-chan struct{}) {
	ticker := time.NewTicker(supervisorRenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			_ = s.store.RenewRunLease(context.Background(), run.RunID, s.owner, time.Now().Add(supervisorLeaseTTL))
		}
	}
}

func (s *Supervisor) finish(ctx context.Context, run store.TaskRun, runErr error) {
	status := domain.TaskCompleted
	if runErr != nil {
		status = domain.TaskFailed
	}
	if ctx.Err() != nil {
		status = domain.TaskCancelled
		runErr = errors.Join(runErr, ctx.Err())
	}
	finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var persistErr error
	if ctx.Err() != nil {
		persistErr = errors.Join(persistErr, s.store.CancelQueuedAgents(finalCtx, run.TaskID))
		for _, approval := range pendingApprovals(finalCtx, s.store, run.TaskID) {
			persistErr = errors.Join(persistErr, s.store.ResolveApproval(finalCtx, approval.ID, false, "task stopped"))
		}
	}
	persistErr = errors.Join(persistErr,
		s.store.UpdateTaskStatus(finalCtx, run.TaskID, status),
		s.store.FinishTaskRun(finalCtx, run.RunID, status, errorString(runErr)),
	)
	if persistErr != nil {
		s.mu.Lock()
		s.errs = append(s.errs, fmt.Errorf("task %s: %w", run.TaskID, persistErr))
		s.mu.Unlock()
	}
}

// Recover reclaims expired runs after a restart. Cancel-requested runs are
// marked cancelled instead of re-executed.
func (s *Supervisor) Recover(ctx context.Context) ([]store.TaskRun, error) {
	runs, err := s.store.RecoverExpiredRuns(ctx, s.owner, time.Now())
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		if run.CancelRequested {
			finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			statusErr := errors.Join(
				s.store.UpdateTaskStatus(finalCtx, run.TaskID, domain.TaskCancelled),
				s.store.FinishTaskRun(finalCtx, run.RunID, domain.TaskCancelled, "cancel requested"),
				s.store.CancelQueuedAgents(finalCtx, run.TaskID),
			)
			cancel()
			if statusErr != nil {
				s.mu.Lock()
				s.errs = append(s.errs, fmt.Errorf("task %s: %w", run.TaskID, statusErr))
				s.mu.Unlock()
			}
			continue
		}
		var specs []agent.Spec
		if err := json.Unmarshal(run.PlanJSON, &specs); err != nil {
			finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = s.store.UpdateTaskStatus(finalCtx, run.TaskID, domain.TaskFailed)
			_ = s.store.FinishTaskRun(finalCtx, run.RunID, domain.TaskFailed, err.Error())
			cancel()
			s.mu.Lock()
			s.errs = append(s.errs, fmt.Errorf("task %s: %w", run.TaskID, err))
			s.mu.Unlock()
			continue
		}
		task, err := s.store.GetTask(ctx, run.TaskID)
		if err != nil {
			finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = s.store.UpdateTaskStatus(finalCtx, run.TaskID, domain.TaskFailed)
			_ = s.store.FinishTaskRun(finalCtx, run.RunID, domain.TaskFailed, err.Error())
			cancel()
			s.mu.Lock()
			s.errs = append(s.errs, fmt.Errorf("task %s: %w", run.TaskID, err))
			s.mu.Unlock()
			continue
		}
		workspace, err := s.store.GetWorkspace(ctx, task.WorkspaceID)
		if err != nil {
			finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = s.store.UpdateTaskStatus(finalCtx, run.TaskID, domain.TaskFailed)
			_ = s.store.FinishTaskRun(finalCtx, run.RunID, domain.TaskFailed, err.Error())
			cancel()
			s.mu.Lock()
			s.errs = append(s.errs, fmt.Errorf("task %s: %w", run.TaskID, err))
			s.mu.Unlock()
			continue
		}
		s.launch(run, specs, agent.NewGateRunner(s.store, s.runner, run.TaskID, workspace.Root, gatePollInterval))
	}
	return runs, nil
}

func pendingApprovals(ctx context.Context, s *store.Store, taskID string) []domain.ApprovalRequest {
	pending, err := s.ListPendingApprovals(ctx, taskID)
	if err != nil {
		return nil
	}
	return pending
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *Supervisor) Cancel(ctx context.Context, workspaceID, taskID string) error {
	task, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.WorkspaceID != workspaceID {
		return fmt.Errorf("task does not belong to workspace")
	}
	if err := s.store.RequestTaskCancel(ctx, taskID); err != nil {
		return err
	}
	for _, approval := range pendingApprovals(ctx, s.store, taskID) {
		_ = s.store.ResolveApproval(ctx, approval.ID, false, "task cancelled")
	}
	s.mu.Lock()
	if cancel := s.jobs[taskID]; cancel != nil {
		cancel()
	}
	s.mu.Unlock()
	return nil
}

func (s *Supervisor) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Supervisor) Close(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	for _, cancel := range s.jobs {
		cancel()
	}
	s.mu.Unlock()
	waitErr := s.Wait(ctx)
	s.PTY().CloseAll()
	s.LSP().Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	return errors.Join(waitErr, errors.Join(s.errs...))
}

// Shutdown stops the supervisor without cancelling in-flight runs: new starts
// are rejected, the caller waits up to the context deadline, and unfinished
// runs keep their leases so a later daemon recovers them. Never use Close for
// planned daemon restarts: Close cancels every job and persists cancellation.
// PTY shells are child processes of this daemon and cannot survive a restart,
// so they are always terminated here.
func (s *Supervisor) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	waitErr := s.Wait(ctx)
	s.PTY().CloseAll()
	s.LSP().Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	return errors.Join(waitErr, errors.Join(s.errs...))
}

type durablePersistedRunner struct {
	store  *store.Store
	runner agent.Runner
	taskID string
}

func (r *durablePersistedRunner) Run(ctx context.Context, spec agent.Spec, emit func(agent.Event)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.store.UpdateAgentStatus(ctx, r.taskID, spec.ID, domain.AgentRunning); err != nil {
		return err
	}
	task, err := r.store.GetTask(ctx, r.taskID)
	workspaceID := ""
	if err == nil {
		workspaceID = task.WorkspaceID
	}
	err = r.runner.Run(ctx, spec, func(event agent.Event) {
		emit(event)
		event.AgentID = spec.ID
		payload, encodeErr := json.Marshal(event)
		if encodeErr != nil {
			return
		}
		_ = r.store.AppendEvent(ctx, domain.EventEnvelope{WorkspaceID: workspaceID, TaskID: r.taskID, AgentID: spec.ID, Type: domain.EventOutput, Payload: payload})
	})
	status := domain.AgentCompleted
	if err != nil {
		status = domain.AgentFailed
	}
	if ctx.Err() != nil {
		status = domain.AgentCancelled
		err = errors.Join(err, ctx.Err())
	}
	finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return errors.Join(err, r.store.UpdateAgentStatus(finalCtx, r.taskID, spec.ID, status))
}
