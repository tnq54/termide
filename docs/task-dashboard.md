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
- Dashboard hiện là read-only: chưa start/cancel task, chưa có live push hoặc
  automatic reconnect. `r` là thao tác refresh rõ ràng.
