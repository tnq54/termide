# Server-Push Events + PTY Streaming Design

## Intent

Event server-push thật và terminal tương tác, không phá khung lockstep
request/response hiện có.

## Decisions

- **Event push dùng dedicated stream connection.** Client mở connection thứ
  hai (`ssh ... termide-server --stdio`), gửi `subscribe_stream`, rồi chỉ đọc:
  server gửi một frame `replay` bù cursor, sau đó một frame `event` cho mỗi
  event mới. Không interleave request/response trên cùng connection, không
  cần demux.
- **PTY multiplex qua request/response hiện có.** Session PTY sống trong
  daemon (`open/write/read/resize/close`); TUI poll `read` mỗi 200ms. Không
  cần full-duplex, tương thích Bridge/Remote hiện tại.
- **Dashboard ưu tiên stream, fallback wait-loop.** Có dialer → stream;
  dial lỗi/thiếu dialer → vòng long-poll 10s hiện tại.
- **Terminal panel chuyển sang PTY tương tác.** `run_command` giữ lại cho
  automation và one-shot.

## Protocol

- `subscribe_stream` `{workspace_id, after_sequence}` → server trả `replay`
  một lần rồi `event` liên tục. Client đóng connection để dừng.
- `pty.open {rows,cols}` → `{pty_id}`; `pty.write {pty_id, data}`;
  `pty.read {pty_id, after}` → `{data, next}`; `pty.resize {rows,cols}`;
  `pty.close {pty_id}`. `data` là `[]byte` (JSON base64).
- Mọi message kiểm tra workspace identity; worktree/shell chạy trong
  workspace root của daemon user (tương đương quyền SSH).

## PTY registry

- `PTYRegistry` sống ở Supervisor (daemon-scoped, sống qua reconnect) và
  share cho durable handler; session handler legacy giữ registry riêng.
- Output ring 256 KiB/session với offset tuyệt đối; `read(after)` trả phần mới.
- Reap session idle quá 30 phút khi mở session mới.
- Dùng `creack/pty` (đã có trong module cache).

## Safety and failure handling

- Stream chỉ đọc sau upgrade; write lỗi hoặc ctx hủy thì kết thúc, không ảnh
  hưởng connection khác.
- Bridge phát hiện `subscribe_stream` và chuyển sang bơm một chiều
  control→session.
- Cancel long-poll/request vẫn đóng transport như cũ; stream connection đóng
  độc lập, không ảnh hưởng interactive connection.
- PTY không qua command classifier (shell tương tác = quyền SSH); lệnh
  one-shot `run_command` giữ nguyên gate.
- UI: stream lỗi → fallback wait-loop một lần rồi dừng báo lỗi; PTY poll lỗi
  → đóng session, giữ nội dung đã nhận.

## Success criteria

1. Client mở stream, server push event mới trong ≤1s mà không cần request thêm.
2. Đóng stream không ảnh hưởng interactive connection.
3. PTY: gõ `echo hi` thấy output; resize đổi kích thước; reconnect thấy output
   cũ qua `read`; idle 30 phút bị reap.
4. Dashboard live qua stream khi có dialer, fallback wait-loop khi không.
5. Full suite, race, vet, cross-build pass.
