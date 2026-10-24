# Task dashboard

Dashboard hoạt động khi client mở workspace từ server có protocol `tasks.*`.

## Phím tắt

| Phím | Thao tác |
|---|---|
| `↑` / `↓` | Chọn task; cuộn nội dung trong detail |
| `PageUp` / `PageDown` | Cuộn detail theo trang |
| `Home` / `End` | Đầu/cuối nội dung detail đã tải |
| `s` | Chạy task queued đang mở |
| `c` | Yêu cầu hủy task đang chạy trong phiên hiện tại |
| `Enter` | Mở agent status và event log của task |
| `r` | Refresh task list, hoặc refresh detail/log incremental |
| `l` | Tải trang task kế tiếp |
| `Esc` | Quay lại dashboard từ detail |
| `e` | Mở remote editor |
| `n` | Tạo task mới |

Task list được phân trang theo SQLite `rowid`, không hiển thị timestamp giả vì
schema hiện tại chưa lưu thời gian tạo/cập nhật. Detail hiển thị status, role /
status của agents và output event.

Danh sách cuộn theo task được chọn. Detail cuộn chung thông tin task, agents và
log; dùng `End` để xem log cuối đã tải. Thanh trạng thái được giữ ở dưới cùng,
kể cả khi danh sách dài. Chiều rộng tính theo ô terminal cho CJK/emoji.
Refresh khi chưa kết nối sẽ báo lỗi thay vì gửi request.

Start gửi nguyên mô tả task cho agent `coder` với ID riêng cho mỗi lần chạy.
Sau phản hồi Start thành công, client tự tải snapshot để hiển thị trạng thái
thực tế và log mới; không tự gán trạng thái `running`. Mỗi snapshot chỉ tải một
trang event; tiếp tục nhấn `r` nếu còn log chưa tải.

## Giới hạn an toàn

- Mọi truy vấn đều giới hạn theo `workspace_id`; task cùng ID ở workspace khác
  không thể được mở qua dashboard.
- Mỗi snapshot trả tối đa 50 event và 2 MiB payload.
- Output được loại bỏ control characters trước khi render trong TUI.
- Client kiểm tra workspace/task của phản hồi, agent và event; từ chối event
  có sequence lặp/đảo thứ tự và cursor không khớp. Refresh thất bại giữ lại
  danh sách và cursor cũ để tải tiếp hoặc thử lại.
- Dashboard hỗ trợ `s` trong task detail để start task đang ở trạng thái `queued`.
Server nhận agent plan mặc định gồm một agent `coder`, chạy bằng OpenCode runner
được cấu hình qua `termide-server --opencode`. Server nhận quyền chạy task bằng
transaction trước khi trả xác nhận, rồi chạy agent nền trong phiên stdio.
Request Start trùng bị từ chối; mỗi agent production dùng worktree riêng.

Start dùng timeout request 15 giây, không chờ agent hoàn tất. Có thể refresh
snapshot và dùng editor khi agent đang chạy. Nhấn `r` để lấy trạng thái/log mới.

Khi stream SSH đóng, server hủy các job của phiên và chờ runtime ghi trạng thái
cuối trước khi đóng store. Chưa có detached supervisor: task không tiếp tục
chạy qua reconnect. Cơ chế này được kiểm thử bằng runner giả tuân theo context;
chưa bảo đảm dừng toàn bộ cây process con của OpenCode khi ngắt kết nối.

## Hủy task

Trong detail, nhấn `c` khi task đang chạy, planning, blocked hoặc chờ approval.
Server kiểm tra task thuộc workspace và có job do phiên hiện tại sở hữu trước
khi hủy context. Không hủy task queued hoặc job của phiên khác.

Phản hồi `tasks.cancel_requested` chỉ xác nhận đã gửi yêu cầu; runtime ghi
`cancelled` sau khi runner dừng. Client tải lại snapshot ngay; nếu process chưa
dừng xong, nhấn `r` để xem trạng thái mới. Lỗi request giữ nguyên trạng thái đã
biết và hiển thị lỗi. Giới hạn dừng process con OpenCode nêu trên vẫn áp dụng.

Client có thể dùng request `subscribe` với `AfterSequence` để lấy replay event
đã lưu trong SQLite theo đúng workspace; event trả về được giữ nguyên thứ tự
sequence. Đây là reconnect/replay một lần, chưa phải live push.

`r` vẫn là thao tác refresh rõ ràng cho task snapshot. Job đang chạy vẫn bị hủy
khi stdio session đóng; durable supervisor qua reconnect chưa có.
