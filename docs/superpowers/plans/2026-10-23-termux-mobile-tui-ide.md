# Termux Mobile TUI IDE Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Xây dựng một IDE TUI mobile-first trên Termux để mở project Python trên VPS, chỉnh sửa code, chạy terminal và điều phối nhiều OpenCode agent song song qua SSH.

**Architecture:** Tách thành `client` chạy trên Android/Termux và `server` chạy trên VPS. Client render TUI, editor và thao tác người dùng; server giữ session/task/agent, PTY, Git worktree và OpenCode process để task không mất khi SSH ngắt. Hai bên trao đổi event có sequence number qua SSH tunnel, cho phép reconnect và replay event.

**Tech Stack:** Go 1.24+; Bubble Tea, Lip Gloss và Bubbles cho TUI; `golang.org/x/crypto/ssh` và `github.com/creack/pty` cho SSH/PTY; SQLite WAL trên VPS cho state; `go-tree-sitter` cho syntax highlighting; LSP JSON-RPC cho Pyright hoặc `python-lsp-server`; OpenCode CLI; Git worktree; `go test`, `go vet`, race detector và integration tests với Docker/SSH test server.

**Spec:** `docs/superpowers/specs/2026-10-23-termux-mobile-tui-ide-design.md`

## Global Constraints

- Android/Termux là lớp giao diện và điều khiển; VPS là nơi chứa source code, chạy OpenCode CLI, LSP và test.
- Portrait là chế độ chính; landscape là chế độ mở rộng.
- Không phụ thuộc vào `vim` hoặc `nvim` cho editor tích hợp.
- Ưu tiên Python trong phiên bản đầu.
- Mỗi agent dùng Git branch/worktree riêng.
- Agent tiếp tục chạy khi TUI tạm ngắt kết nối.
- API key không được ghi vào TUI log, command echo hoặc diff.
- Không chạy agent bằng root.
- Thao tác nguy hiểm phải chờ approval.
- Mọi event có task ID, agent ID, workspace ID và sequence number.

## Review Focus

- **Mất kết nối giữa chừng:** reconnect phải khôi phục trạng thái và replay đúng event còn thiếu; test thuộc Task 3.
- **Terminal nhỏ/resize:** portrait không làm mất input hoặc render vượt vùng; test thuộc Task 4.
- **Lệnh nguy hiểm bị ngụy trang:** command classifier phải yêu cầu approval cho tác động rộng hoặc path ngoài workspace; test thuộc Task 6.
- **Hai agent sửa cùng file:** merge không được tự ghi đè và phải tạo conflict có thể review; test thuộc Task 7.
- **LSP/tooling không có sẵn:** project vẫn mở được, báo degraded capability thay vì crash; test thuộc Task 8.

---

### Task 1: Khởi tạo monorepo và contract domain

**Files:**
- Create: `go.mod`
- Create: `cmd/termide/main.go`
- Create: `cmd/termide-server/main.go`
- Create: `internal/domain/workspace.go`
- Create: `internal/domain/task.go`
- Create: `internal/domain/agent.go`
- Create: `internal/domain/events.go`
- Create: `internal/domain/errors.go`
- Create: `internal/domain/*_test.go`
- Create: `Makefile`
- Create: `.gitignore`

**Interfaces:**
- Produces `Workspace`, `Task`, `Agent`, `EventEnvelope`, `AgentStatus`, `TaskStatus`, `ApprovalRequest` và các ID kiểu string để mọi task sau dùng chung.

- [ ] **Step 1: Viết failing tests** cho serialize/deserialize domain objects, chuyển trạng thái hợp lệ và từ chối trạng thái không hợp lệ.
- [ ] **Step 2: Chạy `go test ./internal/domain -v`** và xác nhận test fail vì type/transition chưa tồn tại.
- [ ] **Step 3: Implement domain types** với JSON tags ổn định, timestamp UTC và transition function trả error thay vì panic.
- [ ] **Step 4: Chạy `go test ./...` và `go vet ./...`**; xác nhận PASS.
- [ ] **Step 5: Commit** `chore: initialize termide domain contracts`.

### Task 2: Persistence cho workspace, task, agent và event journal

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/sqlite.go`
- Create: `internal/store/migrations/001_initial.sql`
- Create: `internal/store/sqlite_test.go`

**Interfaces:**
- Consumes domain types từ Task 1.
- Produces `Store.CreateWorkspace`, `Store.CreateTask`, `Store.UpsertAgent`, `Store.AppendEvent`, `Store.EventsAfter`, `Store.GetTask` và `Store.ListWorkspaces`.

- [ ] **Step 1: Viết tests** cho persistence, unique IDs, event sequence tăng đơn điệu, query event sau sequence và khôi phục task sau process restart.
- [ ] **Step 2: Chạy `go test ./internal/store -v`** và xác nhận fail.
- [ ] **Step 3: Implement SQLite WAL schema** gồm workspace, task, agent, event và approval; transaction bảo đảm event sequence không trùng.
- [ ] **Step 4: Test restart và concurrent append** bằng `go test -race ./internal/store -v`.
- [ ] **Step 5: Commit** `feat: persist workspace and task state`.

### Task 3: SSH transport, server protocol và reconnect

**Files:**
- Create: `internal/protocol/messages.go`
- Create: `internal/protocol/codec.go`
- Create: `internal/server/session.go`
- Create: `internal/client/remote.go`
- Create: `internal/transport/ssh.go`
- Create: `internal/transport/replay.go`
- Create: `internal/transport/*_test.go`
- Modify: `cmd/termide-server/main.go`

**Interfaces:**
- Produces request/response/event protocol với `Hello`, `Subscribe`, `CreateTask`, `Approve`, `Cancel`, `RunCommand`, `EventEnvelope`.
- `RemoteClient.Connect(ctx)`, `RemoteClient.Subscribe(ctx, workspaceID, afterSequence)` và `RemoteClient.Call(ctx, request)`.

- [ ] **Step 1: Viết protocol tests** cho framing, malformed JSON, unknown message type và event ordering.
- [ ] **Step 2: Viết reconnect integration test** với fake server: gửi event 1, ngắt client, phát event 2-3, reconnect từ sequence 1 và assert nhận 2-3 đúng thứ tự.
- [ ] **Step 3: Chạy test và xác nhận fail.**
- [ ] **Step 4: Implement length-delimited JSON protocol trên SSH channel**, handshake version, workspace/session identity và replay từ SQLite.
- [ ] **Step 5: Chạy `go test -race ./internal/protocol ./internal/transport ./internal/server`** và xác nhận PASS.
- [ ] **Step 6: Commit** `feat: add reconnectable ssh control protocol`.

### Task 4: TUI shell mobile-first và navigation

**Files:**
- Create: `internal/ui/model.go`
- Create: `internal/ui/views.go`
- Create: `internal/ui/keymap.go`
- Create: `internal/ui/layout.go`
- Create: `internal/ui/statusbar.go`
- Create: `internal/ui/ui_test.go`
- Modify: `cmd/termide/main.go`

**Interfaces:**
- Consumes `RemoteClient` từ Task 3.
- Produces view modes `Dashboard`, `Editor`, `Agents`, `Terminal`, `Diff`, `Approvals`, command palette và keyboard map.

- [ ] **Step 1: Viết model tests** cho key navigation, `WindowSizeMsg` portrait/landscape và không render panel ngoài bounds.
- [ ] **Step 2: Thêm snapshot-like assertions** cho portrait: chỉ một panel nội dung chính; landscape: editor + side panel khi đủ width.
- [ ] **Step 3: Chạy `go test ./internal/ui -v`** và xác nhận fail.
- [ ] **Step 4: Implement Bubble Tea model**, mobile-first layout, status bar, help overlay và command palette; không đưa network calls vào `View`.
- [ ] **Step 5: Chạy `go test -race ./internal/ui -v`** và kiểm tra thủ công bằng `go run ./cmd/termide` trong terminal 40x12 và 120x35.
- [ ] **Step 6: Commit** `feat: add mobile-first tui shell`.

### Task 5: Remote project browser và editor buffer

**Files:**
- Create: `internal/fs/remote_fs.go`
- Create: `internal/editor/buffer.go`
- Create: `internal/editor/manager.go`
- Create: `internal/editor/search.go`
- Create: `internal/editor/diff.go`
- Create: `internal/editor/*_test.go`
- Modify: `internal/ui/views.go`

**Interfaces:**
- `RemoteFS.List`, `RemoteFS.ReadFile`, `RemoteFS.WriteFile`.
- `Buffer.ApplyEdit`, `Buffer.Undo`, `Buffer.Redo`, `Buffer.Save`, `Buffer.Dirty`.
- `EditorManager.Open`, `EditorManager.Close`, `EditorManager.Active`, `EditorManager.Find`.

- [ ] **Step 1: Viết tests** cho UTF-8, empty file, newline cuối file, undo/redo nhiều bước, dirty state và path traversal bị từ chối.
- [ ] **Step 2: Chạy `go test ./internal/editor ./internal/fs -v`** và xác nhận fail.
- [ ] **Step 3: Implement buffer/editor** với line-based editing, tab/buffer state, remote file operations và path canonicalization trong workspace root.
- [ ] **Step 4: Implement project-wide search** streaming kết quả, giới hạn output và hủy bằng context.
- [ ] **Step 5: Chạy `go test -race ./internal/editor ./internal/fs -v`** và xác nhận PASS.
- [ ] **Step 6: Commit** `feat: add remote editor buffers and search`.

### Task 6: Approval policy, audit log và terminal PTY

**Files:**
- Create: `internal/security/classifier.go`
- Create: `internal/security/policy.go`
- Create: `internal/security/audit.go`
- Create: `internal/terminal/pty.go`
- Create: `internal/security/*_test.go`
- Create: `internal/terminal/*_test.go`
- Modify: `internal/protocol/messages.go`
- Modify: `internal/ui/views.go`

**Interfaces:**
- `Classifier.Classify(command, cwd) Decision` với `safe`, `approval_required`, `blocked`.
- `ApprovalManager.Request`, `ApproveOnce`, `ApproveForTask`, `Deny`.
- `PTY.Start`, `Resize`, `Input`, `Output`, `Close`.

- [ ] **Step 1: Viết tests** cho `rm -rf`, `sudo`, `chmod`, `git push --force`, deploy/restart, path ngoài root, shell substitution và command an toàn như `pytest`/`git status`.
- [ ] **Step 2: Viết PTY tests** cho resize, EOF, output truncation và cancel.
- [ ] **Step 3: Chạy test và xác nhận fail.**
- [ ] **Step 4: Implement policy deny-by-default cho unknown/high-impact commands**, command nguyên bản trong audit record, secret redaction và approval event.
- [ ] **Step 5: Implement PTY backend** chạy dưới user VPS hiện tại, giới hạn scrollback và context cancellation.
- [ ] **Step 6: Chạy `go test -race ./internal/security ./internal/terminal -v`** và xác nhận PASS.
- [ ] **Step 7: Commit** `feat: add guarded terminal and approvals`.

### Task 7: Git worktree isolation và merge review

**Files:**
- Create: `internal/git/worktree.go`
- Create: `internal/git/merge.go`
- Create: `internal/git/diff.go`
- Create: `internal/git/*_test.go`
- Modify: `internal/domain/agent.go`

**Interfaces:**
- `WorktreeManager.Create(taskID, agentID, baseRef)`, `Remove`, `Status`.
- `MergeManager.Preview`, `Merge`, `Abort`.
- `DiffManager.ForWorktree`.

- [ ] **Step 1: Tạo temporary Git repository trong test** với hai worktree và commit độc lập.
- [ ] **Step 2: Viết tests** chứng minh worktree độc lập, diff có file list, fast-forward merge và conflict không tự ghi đè.
- [ ] **Step 3: Chạy `go test ./internal/git -v`** và xác nhận fail.
- [ ] **Step 4: Implement commands bằng `exec.CommandContext`**, không shell interpolation, validate repository root và giữ conflict state để review.
- [ ] **Step 5: Chạy `go test -race ./internal/git -v`** và xác nhận PASS.
- [ ] **Step 6: Commit** `feat: isolate agents with git worktrees`.

### Task 8: Python tooling và LSP bridge

**Files:**
- Create: `internal/python/discover.go`
- Create: `internal/python/commands.go`
- Create: `internal/lsp/client.go`
- Create: `internal/lsp/jsonrpc.go`
- Create: `internal/python/*_test.go`
- Create: `internal/lsp/*_test.go`
- Modify: `internal/editor/manager.go`

**Interfaces:**
- `PythonEnvironment.Discover(workspaceRoot)`, `Interpreter`, `Tool(name)`.
- `PythonRunner.Command(tool, args)`.
- `LSPClient.Initialize`, `Open`, `Change`, `Request`, `Close`.

- [ ] **Step 1: Viết discovery tests** cho `.venv`, `venv`, Poetry, uv và project không có environment; assert degraded status thay vì error fatal.
- [ ] **Step 2: Viết JSON-RPC/LSP tests** cho Content-Length framing, diagnostics, completion, definition và cancellation.
- [ ] **Step 3: Chạy test và xác nhận fail.**
- [ ] **Step 4: Implement discovery không chạy command không tin cậy**, ưu tiên project config và tool tồn tại trên VPS.
- [ ] **Step 5: Implement LSP bridge** quản lý process Pyright hoặc `python-lsp-server`, map diagnostics/completion/definition/rename/format/code action vào editor.
- [ ] **Step 6: Chạy `go test -race ./internal/python ./internal/lsp -v`** và xác nhận PASS; chạy integration test nếu binary LSP có sẵn.
- [ ] **Step 7: Commit** `feat: integrate python tooling and lsp`.

### Task 9: Server task runtime và OpenCode multi-agent orchestration

**Files:**
- Create: `internal/agent/runtime.go`
- Create: `internal/agent/opencode.go`
- Create: `internal/agent/planner.go`
- Create: `internal/agent/scheduler.go`
- Create: `internal/agent/redact.go`
- Create: `internal/agent/*_test.go`
- Modify: `internal/store/store.go`
- Modify: `cmd/termide-server/main.go`

**Interfaces:**
- `Runtime.CreateTask`, `Runtime.StartTask`, `Runtime.CancelTask`, `Runtime.GetSnapshot`.
- `OpenCodeRunner.Start(ctx, AgentSpec)`, `Stop`, `StreamEvents`.
- `Scheduler.Schedule(plan)`, `Scheduler.Cancel(agentID)`.

- [ ] **Step 1: Viết fake OpenCode runner tests** cho planner tạo independent agents, scheduler chạy song song, event persistence và retry giới hạn.
- [ ] **Step 2: Viết test** chứng minh API key/secret bị redact khỏi command/log/error.
- [ ] **Step 3: Chạy test và xác nhận fail.**
- [ ] **Step 4: Implement runner** bằng process supervisor và environment secret ở VPS; không truyền secret qua event stream.
- [ ] **Step 5: Implement planner/scheduler** tạo worktree Task 7, stream output Task 3, tự động thao tác safe và pause ở approval.
- [ ] **Step 6: Implement reconnect-safe supervisor** để child process sống qua client disconnect và resume snapshot.
- [ ] **Step 7: Chạy `go test -race ./internal/agent ./internal/server ./internal/store -v`** và xác nhận PASS.
- [ ] **Step 8: Commit** `feat: orchestrate persistent parallel opencode agents`.

### Task 10: Tích hợp dashboard, diff, approval và task lifecycle

**Files:**
- Create: `internal/ui/dashboard.go`
- Create: `internal/ui/agent_panel.go`
- Create: `internal/ui/approval_panel.go`
- Create: `internal/ui/diff_panel.go`
- Create: `internal/ui/task_flow.go`
- Create: `internal/ui/integration_test.go`
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/layout.go`

**Interfaces:**
- Consumes event stream, editor, terminal, approval, diff và task APIs từ Tasks 3–9.
- Produces complete TUI flow: chọn VPS/project → tạo task → agent chạy → approval → diff → merge.

- [ ] **Step 1: Viết integration test** với fake remote server cho full lifecycle và assert các panel nhận đúng state.
- [ ] **Step 2: Viết test** cho portrait single-focus và landscape split view trong lúc agent vẫn stream log.
- [ ] **Step 3: Chạy test và xác nhận fail.**
- [ ] **Step 4: Implement dashboard và panel navigation**, output collapse/filter, agent status, command preview, approval actions và diff review.
- [ ] **Step 5: Implement task lifecycle** gồm create/start/pause/cancel/retry/merge; conflict phải chuyển sang trạng thái cần review.
- [ ] **Step 6: Chạy `go test -race ./internal/ui ./...`** và kiểm tra thủ công màn hình nhỏ.
- [ ] **Step 7: Commit** `feat: integrate task-based ide workspace`.

### Task 11: SSH deployment, configuration và end-to-end validation

**Files:**
- Create: `configs/example.toml`
- Create: `deploy/termide-server.service`
- Create: `scripts/install-server.sh`
- Create: `scripts/run-e2e.sh`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `README.md`
- Create: `docs/architecture.md`
- Create: `tests/e2e/*`

**Interfaces:**
- Config hỗ trợ SSH host aliases, server binary path, workspace root, LSP command, OpenCode command, retention limits và approval policy.

- [ ] **Step 1: Viết config tests** cho default an toàn, missing secret reference, invalid path và unknown field.
- [ ] **Step 2: Viết E2E harness** dựng fake VPS SSH server, repository Python mẫu, fake OpenCode CLI, task hai agents và simulated disconnect.
- [ ] **Step 3: Chạy `go test ./internal/config ./tests/e2e -v`** và xác nhận fail.
- [ ] **Step 4: Implement config loader và install/service scripts**; secret chỉ lấy từ environment hoặc file permission `0600` trên VPS.
- [ ] **Step 5: Implement E2E assertions**: project open, editor read/write, LSP degraded/full, parallel agents, approval, diff, conflict, reconnect và merge.
- [ ] **Step 6: Chạy toàn bộ verification:**
  - `go test ./...`
  - `go test -race ./...`
  - `go vet ./...`
  - `go build ./cmd/termide ./cmd/termide-server`
  - `bash scripts/run-e2e.sh`
- [ ] **Step 7: Commit** `docs: document termide deployment and workflows`.

## Milestone order

1. **Foundation:** Tasks 1–3 — domain, persistence, SSH protocol/reconnect.
2. **Usable TUI:** Tasks 4–5 — navigation, remote editor, search.
3. **Safe execution:** Tasks 6–7 — terminal, approval, Git isolation.
4. **Developer intelligence:** Tasks 8–9 — Python/LSP and OpenCode orchestration.
5. **Complete IDE:** Tasks 10–11 — integrated lifecycle, deployment and E2E.

Mỗi milestone phải chạy được test suite của milestone trước khi bắt đầu milestone tiếp theo. Không thêm provider, ngôn ngữ hoặc GUI ngoài phạm vi khi các tiêu chí E2E trên chưa đạt.
