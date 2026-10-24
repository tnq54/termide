# Terminal tương tác (PTY)

Panel terminal (`t` từ dashboard hoặc task detail) mở một shell tương tác
trong workspace root trên VPS qua PTY thật (`creack/pty`, `TERM=xterm-256color`).

| Phím | Thao tác |
|---|---|
| Gõ chữ, Enter, Tab, Backspace | Gửi thẳng vào shell |
| ↑ / ↓ | Lịch sử lệnh của shell |
| ← / →, Ctrl+C, Ctrl+D | Gửi mã điều khiển tương ứng |
| PageUp / PageDown | Cuộn output trong TUI |
| Esc | Đóng session, quay về dashboard |

## Cách hoạt động và giới hạn

- Mỗi lần vào panel mở một session PTY mới (`pty.open` với kích thước màn
  hình); TUI poll `pty.read` mỗi 200ms và nối output; Esc gửi `pty.close`.
- Output giữ nguyên byte (kể cả mã màu ANSI); TUI không giả lập cursor VT đầy
  đủ — các ứng dụng fullscreen (vim, top) hiển thị tuyến tính, không chuẩn.
- Ring buffer server 256 KiB/session; client đọc theo offset tuyệt đối nên
  reconnect trong cùng daemon vẫn thấy output cũ qua `pty.read`.
- Session idle quá 30 phút bị thu hồi khi mở session mới. Session là tiến
  trình con của daemon: restart daemon là mất shell, không recover như task.
- Resize màn hình gửi `pty.resize`. Không gõ được khi session chưa mở xong;
  lỗi poll dừng vòng lặp, giữ nội dung đã nhận — Esc rồi `t` để mở lại.
- Lệnh một lần không tương tác (`git status`, `pytest`) vẫn có API
  `run_command` riêng cho automation; panel này dành cho shell tương tác.
