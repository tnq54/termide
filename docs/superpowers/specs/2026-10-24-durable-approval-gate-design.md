# Durable Approval Gate Design

## Intent

Agent spec có command nguy hiểm không được tự chạy: daemon tạm dừng agent,
ghi approval request bền vững vào SQLite, client thấy và approve/deny qua
dashboard; agent chỉ chạy tiếp sau khi được duyệt. Luồng sống qua reconnect
vì mọi trạng thái nằm trong store.

## Non-goals

- Phê duyệt từng tool-call bên trong `opencode run`: kiến trúc hiện tại chỉ
  thấy spec-level command, không thấy từng lệnh con agent thực thi.
- Auto-approve theo policy phức tạp; chỉ có classify safe/approval/blocked.
- Live push: client phát hiện approval pending qua snapshot/replay/`r`.

## State model

Bảng `approvals`: `id` PK, `task_id`, `agent_id`, `command`, `reason`,
`status` (`pending`/`approved`/`denied`). Mỗi agent spec tối đa một approval
pending tại một thời điểm (unique index trên `(task_id, agent_id)` where
pending — thực hiện bằng unique index `(task_id, agent_id, status)` đơn giản
hóa thành kiểm tra trong transaction).

- `pending`: agent ở `waiting_approval`, scheduler goroutine của agent đó
  block có giới hạn poll, tôn trọng ctx cancel.
- `approved`: agent tiếp tục chạy spec.
- `denied`: agent fail, task fail theo ngữ nghĩa scheduler hiện tại
  (agent fail → task failed).

## Flow

1. `Supervisor.Start` (và session path cũ) bọc runner bằng `GateRunner`.
2. `GateRunner.Run` classify `spec.Command` với workspace root:
   - `Safe` → chạy delegate ngay.
   - `Blocked` → trả lỗi, không tạo approval.
   - `ApprovalRequired` → tạo approval record (idempotent theo task+agent),
     set agent `waiting_approval`, emit `EventApproval`, poll store tới khi
     resolved / ctx hủy / task bị cancel.
3. Client gửi `MessageApprove{task_id, payload:{id, approved}}`; daemon
   resolve atomic pending→terminal, emit `EventApproval`.
4. Dashboard: panel approvals liệt kê pending của task, phím `a` approve,
   `d` deny; refresh/replay thấy event mới.

## Safety

- Resolve chỉ chuyển `pending` → terminal một lần (conditional UPDATE);
  approve/deny trùng bị từ chối.
- Cancel task trong lúc chờ approval: poll thoát theo ctx, agent
  `cancelled`, approval pending còn lại được đánh `denied` với lý do
  `task cancelled` để không kẹt pending vĩnh viễn.
- Không log secret: command hiển thị nguyên văn trong dashboard của chủ
  workspace (chủ đã có quyền đọc worktree).
- `RequestID` và identity checks giữ nguyên như các message khác.

## Success criteria

1. Spec safe chạy không cần approval; spec `rm -rf` dừng ở pending và agent
   `waiting_approval`.
2. Approve qua message thứ hai → agent chạy tiếp tới completed.
3. Deny → task failed, không chạy command.
4. Duplicate approve bị từ chối; cancel khi đang chờ → agent cancelled.
5. Reconnect vẫn thấy approval pending qua snapshot/replay.
