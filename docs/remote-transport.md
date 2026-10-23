# Transport điều khiển qua SSH

`internal/client.Remote` trao đổi request/response theo frame với
`termide-server --stdio`. Một kết nối xử lý tuần tự nhiều request; client tự
gán `RequestID` và kiểm tra ID phản hồi. Lỗi từ server được trả về caller.

`client.DialSSH(ctx, "dev-vps")` dùng OpenSSH trên máy client:

```sh
ssh -T -o BatchMode=yes -o StrictHostKeyChecking=yes -- dev-vps 'termide-server --stdio'
```

Yêu cầu:

- Có `ssh` trên Termux và alias/host đã cấu hình trong `~/.ssh/config`.
- Host key đã được xác minh và lưu trong `known_hosts`.
- Xác thực không tương tác bằng key hoặc SSH agent.
- `termide-server` nằm trong PATH của shell từ xa.

Không cấp PTY cho kênh protocol. Log server phải đi qua stderr, không ghi xen
text vào stdout. Context truyền cho `DialSSH` quản lý tuổi thọ process SSH.
Hủy một `Call` đóng kết nối vì frame có thể đã gửi/đọc một phần; caller phải
kết nối lại. `Close` đóng pipe và thu hồi process con.

Hiện kiểm thử bao gồm SQLite thật + server handler qua `net.Pipe`, hai request
trên cùng kết nối, hủy khi I/O đang chờ, ID phản hồi sai và pipe process thực.
Chưa xác minh trên VPS thật. TUI đã nối transport để mở workspace và lưu task:

```sh
go run ./cmd/termide --ssh dev-vps --project /home/ubuntu/my-project
```

Đợi trạng thái `connected`, nhấn `n`, nhập mô tả và nhấn Enter. Chỉ khi
server xác nhận, trạng thái mới chuyển thành `task queued`. Workspace được
kiểm tra tồn tại trên VPS; mở lại cùng ID/root không tạo bản ghi trùng.
Không dùng một workspace ID để chuyển sang root khác.

`task queued` chỉ có nghĩa đã lưu SQLite, chưa chạy agent. `start_task` và
event streaming bất đồng bộ chưa được phục vụ bởi vòng stdio.
