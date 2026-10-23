# Editor từ xa

Yêu cầu build: Go 1.25+. Client và server cần cùng bản có protocol `files.*`.

```sh
termide --ssh dev-vps --project /home/ubuntu/project
```

Khi trạng thái là `connected`:

| Phím | Thao tác |
|---|---|
| `e` ở dashboard | Duyệt file project |
| ↑ / ↓ | Chọn file hoặc thư mục |
| Enter | Mở file hoặc vào thư mục |
| Backspace trong danh sách | Thư mục cha |
| Esc trong danh sách | Quay về dashboard |
| Phím mũi tên, Home/End, PageUp/PageDown | Di chuyển trong file |
| Ctrl+S | Lưu snapshot hiện tại lên VPS |
| Ctrl+Z / Ctrl+Y | Undo / redo (tối đa 50 bước undo) |
| Esc trong editor | Quay lại danh sách nếu đã lưu |
| Ctrl+G trong editor | Bỏ thay đổi chưa lưu và quay lại danh sách |

Trong editor các chữ `q`, `a`, `e` là nội dung, không phải phím điều hướng.
Ctrl+C không thoát khi buffer chưa lưu hoặc Save còn đang chờ xác nhận;
Ctrl+S để lưu hoặc Ctrl+G để bỏ thay đổi.
Dấu `*` bên tên file là nội dung chưa lưu. Có thể tiếp tục gõ khi đang Save;
phản hồi Save chỉ xác nhận snapshot đã gửi, không xóa các chỉnh sửa mới hơn.

## Phạm vi hiện tại

- Một file đang mở; text UTF-8 tối đa 1 MiB. Binary và ký tự điều khiển terminal
  không được mở để sửa. Giữ nguyên tab, CRLF và newline cuối file. File dùng
  CR đơn làm newline chưa hỗ trợ; CR trong paste được chuyển sang newline.
- Hiển thị dòng và viewport cuộn ngang/dọc, không đổi code để vừa màn hình.
- Duyệt tối đa 2.000 mục/thư mục; bỏ qua symlink và `.git` trong danh sách.
- Server dùng `os.Root` để giới hạn thao tác trong workspace; từ chối path thoát root.
- Save ghi file tạm rồi rename trong cùng thư mục, giữ permission bits của file cũ.
- Hash nội dung kiểm tra thay đổi xuất hiện trước khi Save. Khi xung đột, nội dung
  trên server và buffer đều được giữ lại; thông báo lỗi ở thanh trạng thái.
- Kiểm tra hash và rename không phải transaction với process ngoài TermIDE.
  Nếu công cụ khác ghi đúng giữa hai thao tác, vẫn có thể xảy ra race. Không tự merge.
- Undo/redo khôi phục text và đặt cursor ở cuối nội dung; chưa khôi phục lựa chọn/cursor cũ.
- Chưa có nhiều tab, syntax highlighting, LSP, tạo/xóa file hay clipboard hệ điều hành.

Sau lỗi kết nối, chưa có tự reconnect; giữ/copy nội dung chưa lưu trước khi đóng client.
