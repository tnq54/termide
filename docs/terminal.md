# Terminal lệnh nhanh

Panel terminal (`t` từ dashboard) chạy một lệnh shell trong workspace root trên
VPS và hiển thị output giới hạn. Không phải PTY đầy đủ: mỗi Enter là một
request/response độc lập, không giữ shell tương tác giữa các lệnh.

| Phím | Thao tác |
|---|---|
| Gõ chữ | Soạn lệnh |
| Enter | Chạy lệnh (tối đa 30 giây) |
| ↑ / ↓, PageUp / PageDown | Cuộn output |
| Esc | Quay về dashboard |

## Giới hạn an toàn

- Chỉ lệnh `Safe` theo classifier mới được chạy (`git status`, `pytest`, `ls`,
  ...). Lệnh rỗng/blocked bị từ chối; lệnh cần approval bị từ chối với hướng
  dẫn mở task thay vì chạy trực tiếp.
- Mỗi lệnh chạy với `Dir` là workspace root; output gộp stdout+stderr, cắt ở
  64 KiB kèm dấu `[truncated]`; exit code khác 0 hiện dòng `[exit N]`.
- Phản hồi sai workspace bị loại; lỗi giữ nguyên lệnh đã soạn để sửa và thử lại.
