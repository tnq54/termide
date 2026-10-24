# Durable Task Supervisor Design

## Intent

Cho phép task OpenCode tiếp tục chạy trên VPS khi SSH/stdio session của Termux bị ngắt, sau đó client reconnect và đọc lại trạng thái cùng event từ SQLite.

## Current Problem

`termide-server --stdio` hiện sở hữu `Handler`, `TaskRuntime` và các goroutine task. `Serve` gọi `Handler.Close` khi stream đóng; `Close` cancel tất cả job rồi chờ runtime ghi trạng thái cuối. Cách này tránh orphan process nhưng đồng nghĩa task không sống qua reconnect. Hơn nữa, chính process chạy `--stdio` thường bị kết thúc cùng SSH command, nên không thể biến `Close` thành no-op để đạt durability.

## Goals

- Tách lifetime của task supervisor khỏi lifetime của một SSH/stdio session.
- Task đã được claim (`running`) tiếp tục được supervisor thực thi sau khi client mất kết nối.
- Trạng thái task/agent và event vẫn được ghi vào SQLite trong lúc không có client.
- Client reconnect có thể lấy snapshot và replay event từ cursor.
- Cancel sau reconnect có thể hủy task đang chạy.
- Chỉ một supervisor được sở hữu một database/workspace process tại một thời điểm.
- Không ghi secret/API key vào event, log hoặc command echo.
- Giữ compatibility với test handler hiện tại bằng chế độ session-scoped explicit.

## Non-goals

- Live push event trong SSH reconnect.
- Đảm bảo task tiếp tục nếu VPS bị reboot hoặc process supervisor bị kill trước khi checkpoint; task sẽ được khôi phục theo policy startup.
- Phân tán supervisor giữa nhiều VPS.
- PTY terminal durable.
- APK Android.

## Architecture

### Durable daemon

`termide-server --daemon --database <path>` mở SQLite, tạo `Supervisor`, nhận task queued/running từ store và chạy scheduler trong process riêng. Daemon giữ lock file cạnh database để tránh hai supervisor dùng chung state. Daemon có control socket local (Unix socket trên Linux) để nhận lệnh start/cancel/status và health check.

### SSH stdio bridge

`termide-server --stdio --database <path>` không trực tiếp sở hữu task nữa. Nó kết nối tới daemon control socket và phục vụ framed protocol trên stdin/stdout. Nếu daemon chưa chạy, bridge trả lỗi rõ ràng; việc tự spawn daemon là phase sau để tránh orphan/permission ambiguity.

`Handler` trong stdio bridge chỉ xử lý request, kiểm tra workspace/task identity và chuyển lệnh tới `Supervisor`. Khi stream đóng, bridge chỉ đóng session; không cancel durable jobs.

### State model

- `queued`: task chưa được supervisor claim.
- `running`: task đã claim và có execution record.
- `cancel_requested`: durable cancellation intent đã ghi store.
- `completed`, `failed`, `cancelled`: terminal states.
- Startup recovery chuyển execution record `running` bị mất lease thành `queued` hoặc `failed` theo policy; phase đầu chọn `queued` để retry an toàn.

### Ownership and recovery

Mỗi execution có `run_id`, `lease_owner`, `lease_until`, `plan_json`, `attempt`. Supervisor renew lease định kỳ. Khi daemon khởi động, các run `running` có lease hết hạn được recover; task chỉ được claim atomically. Cancel chỉ tạo intent trong store và supervisor runtime thực hiện kill process group rồi ghi terminal state.

### Protocol

Các message client hiện tại vẫn giữ nguyên cho dashboard. `start_task` gửi plan tới daemon và trả acknowledgment nhanh. `cancel` gửi cancellation intent và trả `tasks.cancel_requested`. `subscribe`/snapshot dùng event store hiện có. Thêm health/control message nội bộ trên Unix socket, không expose qua workspace protocol.

## Safety and failure handling

- Start phải atomic với task state và execution record; duplicate start không tạo run thứ hai.
- Daemon shutdown graceful: ngừng claim task mới, chờ bounded thời gian, không tự đánh dấu task completed.
- Process cancellation dùng process group trên Linux.
- Store writes khi context session bị hủy dùng `context.WithoutCancel` với timeout ngắn.
- Lock/socket cleanup phải idempotent.
- Payload/event replay giữ giới hạn hiện tại 10.000 event / 2 MiB.

## Success criteria

1. Test đóng stdio session ngay sau `start_task`; runner vẫn tiến tới `completed` và event xuất hiện trong SQLite.
2. Tạo Handler/bridge mới trên cùng database; snapshot/replay thấy trạng thái cuối.
3. Cancel qua bridge thứ hai dừng run đang thuộc daemon và task thành `cancelled`.
4. Hai daemon cùng database bị từ chối bởi lock.
5. Startup recovery xử lý run có lease hết hạn mà không tạo duplicate execution.
6. Full Go test, race, vet, Android client build và Linux server build pass.

## Phased delivery

- Phase 1: durable store schema, execution records, daemon supervisor API và unit/integration tests.
- Phase 2: Unix control socket daemon/bridge, CLI modes và lifecycle tests.
- Phase 3: bridge Handler delegation, reconnect cancel/status integration, docs.
- Phase 4: recovery hardening, crash/restart tests, packaging/deployment notes.
