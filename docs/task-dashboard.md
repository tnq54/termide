# Task dashboard

Dashboard hoạt động khi client mở workspace từ server có protocol `tasks.*`.

## Phím tắt

| Phím | Thao tác |
|---|---|
| `↑` / `↓` | Chọn task |
| `Enter` | Mở agent status và event log của task |
| `r` | Refresh task list, hoặc refresh detail/log incremental |
| `l` | Tải trang task kế tiếp |
| `Esc` | Quay lại dashboard từ detail |
| `e` | Mở remote editor |
| `n` | Tạo task mới |

Task list được phân trang theo SQLite `rowid`, không hiển thị timestamp giả vì
schema hiện tại chưa lưu thời gian tạo/cập nhật. Detail hiển thị status, role /
status của agents và output event.

## Giới hạn an toàn

- Mọi truy vấn đều giới hạn theo `workspace_id`; task cùng ID ở workspace khác
  không thể được mở qua dashboard.
- Mỗi snapshot trả tối đa 50 event và 2 MiB payload.
- Output được loại bỏ control characters trước khi render trong TUI.
- Dashboard hỗ trợ `s` trong task detail để start task đang ở trạng thái `queued`.
Server nhận agent plan mặc định gồm một agent `coder`, chạy bằng OpenCode runner
được cấu hình qua `termide-server --opencode`. Việc chạy vẫn là synchronous trong
request hiện tại; supervisor detached và reconnect-safe child process vẫn là
phần tiếp theo.

Chưa có cancel task, live push hoặc automatic reconnect. `r` là thao tác refresh
rõ ràng.
