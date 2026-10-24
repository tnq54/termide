# Durable Task Supervisor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tách execution task khỏi SSH/stdio session bằng một VPS daemon supervisor bền vững, để task tiếp tục chạy sau reconnect và vẫn cancel/status được.

**Architecture:** `termide-server --daemon` sở hữu SQLite, lock database và task goroutines; `--stdio` chỉ là bridge framed protocol tới daemon qua Unix socket. Execution records trong SQLite lưu run identity, plan và lease để recovery. Dashboard protocol hiện tại tiếp tục dùng snapshot/replay.

**Tech Stack:** Go, SQLite (`modernc.org/sqlite`), Unix domain sockets, framed JSON protocol, Linux process groups, Bubble Tea client.

**Spec:** `docs/superpowers/specs/2026-10-24-durable-task-supervisor-design.md`

## Global Constraints

- Task execution phải tiếp tục khi stdio/SSH session đóng.
- Chỉ một supervisor được sở hữu một database tại một thời điểm.
- Start duplicate phải bị từ chối atomically.
- Cancel phải ghi durable intent và cuối cùng persist terminal status.
- Event replay tối đa 10.000 event / 2 MiB.
- Không ghi secrets/API keys vào event, log hoặc command echo.
- Linux process cancellation dùng process group; non-Linux giữ fallback an toàn.
- Session bridge không cancel durable jobs khi stream đóng.

## Review Focus

- Session đóng ngay sau start: daemon không cancel task và bridge đóng sạch.
- Hai supervisor cùng database: lock từ chối process thứ hai.
- Cancel từ session khác: đúng workspace/task, không để mất cancellation intent.
- Crash/restart với lease hết hạn: recover một lần, không tạo duplicate run.
- Store/daemon shutdown: không báo task completed giả và không nuốt lỗi persist.

---

### Task 1: Durable execution records

**Files:**
- Create: `internal/store/migrations/002_durable_runs.sql`
- Create: `internal/store/runs.go`
- Test: `internal/store/runs_test.go`
- Modify: `internal/store/store.go`

**Interfaces:**
- `ClaimTaskRun(ctx, taskID, planJSON, owner string, leaseUntil time.Time) (Run, error)`
- `GetTaskRun(ctx, taskID) (Run, error)`
- `RequestTaskCancel(ctx, taskID) error`
- `RecoverExpiredRuns(ctx, owner string, now time.Time) ([]Run, error)`
- `FinishTaskRun(ctx, runID string, status domain.TaskStatus, errText string) error`

- [ ] Write failing tests for atomic claim/duplicate rejection, durable cancel intent, and expired lease recovery.
- [ ] Run `go test ./internal/store -run 'TestTaskRun' -count=1`; expect missing schema/API failures.
- [ ] Add `Run` model with `run_id`, `task_id`, `plan_json`, `owner`, `lease_until`, `attempt`, `cancel_requested`, `status`, `error`.
- [ ] Add migration and store methods; claim must update queued task and insert exactly one run transactionally.
- [ ] Run focused tests and full `go test ./internal/store`.
- [ ] Commit `feat: persist durable task runs`.

### Task 2: Supervisor core

**Files:**
- Create: `internal/server/supervisor.go`
- Create: `internal/server/supervisor_test.go`
- Modify: `internal/server/start_task.go`
- Modify: `internal/server/cancel_task.go`

**Interfaces:**
- `NewSupervisor(store *store.Store, runner agent.Runner, owner string) *Supervisor`
- `Supervisor.Start(ctx, workspaceID, taskID string, specs []agent.Spec) (Run, error)`
- `Supervisor.Cancel(ctx, workspaceID, taskID string) error`
- `Supervisor.Snapshot(ctx, taskID string) (domain.TaskSnapshot, error)`
- `Supervisor.Close(ctx context.Context) error`

- [ ] Write a failing integration test where the session context is canceled after `Start`, but the runner completes and store status is `completed`.
- [ ] Run focused test and observe cancellation under current Handler behavior.
- [ ] Implement supervisor-owned context independent of request/session context, with bounded final persistence using `context.WithoutCancel`.
- [ ] Add lease renewal and recovery of expired runs at supervisor startup.
- [ ] Implement durable cancel intent and process cancellation.
- [ ] Run focused supervisor/race tests.
- [ ] Commit `feat: add durable task supervisor core`.

### Task 3: Unix control protocol

**Files:**
- Create: `internal/server/control.go`
- Create: `internal/server/control_test.go`
- Create: `internal/server/lock_unix.go`
- Create: `internal/server/lock_other.go`

**Interfaces:**
- `ControlRequest` with `Type`, `WorkspaceID`, `TaskID`, `Payload`.
- `ControlResponse` with `Type`, `Payload`, `Error`.
- `ServeControl(ctx, listener, supervisor) error`.
- `DialControl(ctx, socketPath) (io.ReadWriteCloser, error)`.
- `AcquireDatabaseLock(path string) (io.Closer, error)`.

- [ ] Write failing tests for framed request/response, malformed request rejection, and second lock rejection.
- [ ] Implement Unix socket server using existing framing conventions; keep control protocol internal.
- [ ] Implement platform fallback that returns an explicit unsupported error.
- [ ] Run control tests and race tests.
- [ ] Commit `feat: add supervisor control socket`.

### Task 4: CLI daemon and stdio bridge

**Files:**
- Modify: `cmd/termide-server/main.go`
- Create: `internal/server/bridge.go`
- Create: `internal/server/bridge_test.go`

**Interfaces:**
- `RunDaemon(ctx, database, socket, runner) error`.
- `RunStdioBridge(ctx, database, socket) error`.

- [ ] Write failing lifecycle test: daemon starts, bridge connects, socket disappears after shutdown.
- [ ] Implement `--daemon`, `--socket`, and explicit bridge delegation.
- [ ] Make bridge report daemon unavailable instead of silently running session-owned jobs.
- [ ] Run Linux lifecycle tests and cross-build client/server.
- [ ] Commit `feat: run durable supervisor daemon`.

### Task 5: Handler delegation and reconnect cancellation

**Files:**
- Modify: `internal/server/handler.go`
- Modify: `internal/server/serve.go`
- Modify: `internal/server/start_task.go`
- Modify: `internal/server/cancel_task.go`
- Create: `internal/server/durable_handler_test.go`

**Interfaces:**
- Handler uses a supervisor/control client for `start_task`, `cancel`, and task status.
- `Handler.Close` closes only session resources; it must not cancel daemon runs.

- [ ] Write failing test that starts through one bridge, closes it, then cancels/reads status through a second bridge.
- [ ] Remove session `cancels` ownership from durable path while preserving explicit legacy session handler tests.
- [ ] Validate workspace/task identity at bridge boundary and preserve existing response types.
- [ ] Run server integration and race tests.
- [ ] Commit `feat: delegate stdio tasks to durable supervisor`.

### Task 6: Dashboard reconnect and documentation

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/dashboard_replay.go`
- Modify: `docs/task-dashboard.md`
- Modify: `docs/superpowers/plans/task-dashboard-progress.md`
- Test: `internal/ui/dashboard_reconnect_test.go`

- [ ] Add test for reconnect after session loss where task remains `running`, then transitions to terminal status after refresh.
- [ ] Ensure replay and snapshot refresh do not invent task status and retain old state on bridge errors.
- [ ] Document daemon startup, socket path, recovery policy, and reconnect/cancel behavior.
- [ ] Run UI tests and full suite.
- [ ] Commit `feat: document durable reconnect workflow`.

### Task 7: Final verification

- [ ] Run `go test ./... -count=1`.
- [ ] Run `go test -race ./internal/...`.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
- [ ] Build `CGO_ENABLED=0 GOOS=android GOARCH=arm64 ./cmd/termide`.
- [ ] Build `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 ./cmd/termide-server`.
- [ ] Review `git status`, commit any remaining docs/test changes, and record known limitations.
