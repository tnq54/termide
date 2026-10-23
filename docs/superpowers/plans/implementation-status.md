# Tiến độ tích hợp hiện tại

## Đã kiểm chứng

- TUI nhận mô tả task bằng bàn phím, gồm tiếng Việt; phím tắt không chặn chữ khi nhập.
- Framing đọc liên tiếp nhiều message không mất frame.
- `termide-server --stdio` phục vụ nhiều request trên stdin/stdout.
- Client giữ một stream, kiểm tra RequestID, đóng transport khi hủy request.
- OpenSSH transport sử dụng known_hosts, không cấp PTY.
- CLI `--ssh HOST --project /absolute/root` nối TUI với remote client.
- `open_workspace` kiểm tra thư mục, cho mở lại cùng ID/root, từ chối đổi root.
- Enter trong composer gửi `create_task`; chỉ hiện `task queued` sau phản hồi server.
- Test tích hợp TUI → net.Pipe → server → SQLite xác nhận task được ghi thật.
- Lỗi gửi task giữ nguyên mô tả và không báo thành công.
- Runtime nhận task bằng transaction: chỉ một lần Start được nhận khi task đang queued.
- Trạng thái running/completed/failed/cancelled được lưu từ kết quả runner;
  lỗi runner không bị nuốt, hủy vẫn ghi được trạng thái cuối.
- Output runner được ghi event journal với ID agent do runtime xác định.
- IsolatedRunner tạo worktree riêng, lưu branch/path trong SQLite và truyền
  WorkingDir cho runner; test hai agent ghi file không thay đổi project gốc.
- OpenCode process runner dùng WorkingDir, thu stderr và thu hồi process khi
  đọc output lỗi. Test dùng executable giả cục bộ, không gọi API thật.

## Chưa hoàn thành

- Chưa chạy trên VPS/Termux thật.
- Vòng stdio chưa dispatch start_task vào runner; chưa có agent chạy từ TUI.
- Worktree đã nối với runtime qua IsolatedRunner; chưa dispatch từ vòng stdio.
- Chưa nối approvals và event replay vào toàn bộ vòng đời task phía TUI.
- Chưa có daemon giữ task sống qua ngắt SSH, editor remote và LSP tích hợp đầy đủ.
- Các mục ghi “complete” trong ledger cũ chỉ thể hiện test nhỏ của từng module,
  không chứng minh toàn bộ task/milestone tương ứng đã hoàn tất.

## Kiểm tra gần nhất

`go test ./... -count=1`, race tests cho client/server/ui/store, `go vet ./...`
và build hai entrypoint đều thành công. Không sử dụng API key trong test tích hợp.
