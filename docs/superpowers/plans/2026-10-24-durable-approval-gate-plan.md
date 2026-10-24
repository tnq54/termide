# Durable Approval Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Chặn agent spec nguy hiểm bằng approval bền vững: safe tự chạy, dangerous chờ duyệt, blocked từ chối.

**Architecture:** Bảng `approvals` trong SQLite; `GateRunner` bọc runner trong supervisor/session path; daemon xử lý `MessageApprove`; dashboard panel duyệt.

**Tech Stack:** Go, SQLite, framed protocol, Bubble Tea.

**Spec:** `docs/superpowers/specs/2026-10-24-durable-approval-gate-design.md`

## Global Constraints

- Mọi trạng thái approval nằm trong SQLite để sống qua reconnect.
- Resolve atomic pending→terminal đúng một lần.
- Không chạy command khi chưa approved; không để approval kẹt pending sau cancel.
- Agent ID duy nhất và thuộc đúng task (guard đã có từ supervisor).

## Review Focus

- Approve đến sau khi agent đã bị cancel: resolve vẫn ghi nhận nhưng agent không chạy lại.
- Hai client approve/deny cùng lúc: chỉ một thắng, còn lại báo lỗi.
- Reconnect giữa lúc pending: snapshot/replay phải thấy approval.
- Spec rỗng/blocked: không tạo approval rác.

---

### Task 1: Durable approval records

**Files:**
- Create: `internal/store/migrations/003_approvals.sql`
- Create: `internal/store/approvals.go`
- Test: `internal/store/approvals_test.go`
- Modify: `internal/store/store.go` (đã glob migrations — kiểm tra)

**Interfaces:**
- `CreateApproval(ctx, ApprovalRequest) error` (chỉ khi chưa có pending cho task+agent)
- `GetApproval(ctx, id) (ApprovalRequest, error)`
- `ListPendingApprovals(ctx, taskID) ([]ApprovalRequest, error)`
- `ResolveApproval(ctx, id string, approved bool, reason string) error` (conditional pending→terminal)

- [ ] Viết test RED: tạo pending, resolve approved, resolve lại bị từ chối, deny, list pending.
- [ ] Chạy test, xác nhận fail vì thiếu API.
- [ ] Thêm migration + methods, phủ `status` trong domain.ApprovalRequest nếu thiếu.
- [ ] Chạy `go test ./internal/store`, commit `feat: persist durable approvals`.

### Task 2: GateRunner

**Files:**
- Create: `internal/agent/gate.go`
- Test: `internal/agent/gate_test.go`

**Interfaces:**
- `NewGateRunner(store, delegate, workspaceRoot, pollInterval) *GateRunner`
- `Run(ctx, spec, emit) error`: Safe→delegate; Blocked→error; ApprovalRequired→tạo record, chờ resolve.

- [ ] Viết test RED: safe chạy ngay; blocked lỗi không tạo record; approval pending rồi approve thì chạy; deny thì lỗi và delegate không chạy.
- [ ] Implement với poll + ctx cancel; agent status waiting_approval/running qua store.
- [ ] Race test, commit `feat: gate dangerous agent specs behind approval`.

### Task 3: Daemon approve endpoint + cancel cleanup

**Files:**
- Modify: `internal/server/approve.go` (mới) + `internal/server/handler.go`
- Test: `internal/server/approve_test.go`
- Modify: `internal/server/supervisor.go` (bọc GateRunner; cancel dọn pending)

**Interfaces:**
- `handleApprove`: parse `{id, approved}`, kiểm tra workspace/task, resolve, emit EventApproval.
- Supervisor bọc mọi runner bằng GateRunner với workspace root.
- `finish`/cancel: pending còn lại → denied `task cancelled`.

- [ ] Test RED: approve/deny qua Handle, duplicate resolve bị từ chối, cancel dọn pending.
- [ ] Implement, chạy suite server, commit `feat: serve durable approval decisions`.

### Task 4: Dashboard approvals panel

**Files:**
- Modify: `internal/ui/approval_panel.go`, panel wiring trong model/dashboard
- Test: `internal/ui/approval_panel_test.go` (mở rộng)

**Interfaces:**
- Panel liệt kê pending của task đang mở; `a` approve, `d` deny qua MessageApprove; refresh/replay giữ nguyên.

- [ ] Test RED: render list pending, approve command gửi đúng message, deny tương tự.
- [ ] Implement wiring, chạy UI tests, commit `feat: approve or deny agents from dashboard`.

### Task 5: Docs + final verification

- [ ] Cập nhật `docs/task-dashboard.md` + progress ledger.
- [ ] `go test ./...`, race, vet, diff-check, cross-build, commit docs.
