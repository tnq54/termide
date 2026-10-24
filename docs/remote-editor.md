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
| Esc trong editor | Cất tab hiện tại, quay lại danh sách nếu đã lưu |
| Ctrl+G trong editor | Bỏ thay đổi chưa lưu và đóng tab |
| Ctrl+N / Ctrl+P trong editor | Tab kế tiếp / tab trước (vòng tròn) |
| Ctrl+J trong editor | Gợi ý hoàn thành từ LSP tại con trỏ (Up/Down chọn, Enter chèn, Esc đóng) |
| Ctrl+O trong editor, hoặc tự động khi gõ `(` `,` | Signature help (overloads, tham số đang nhập; Up/Down chuyển overload, Esc đóng) |
| Ctrl+R trong editor | Đổi tên symbol tại con trỏ (gõ tên mới, Enter áp dụng, Esc hủy) |
| Ctrl+X trong editor | Code action quickfix tại dòng hiện tại (Up/Down chọn, Enter áp dụng, Esc đóng) |
| Ctrl+K trong editor | Hover LSP tại cursor (file `.py`, cần `--lsp-python` ở server) |

Trong editor các chữ `q`, `a`, `e` là nội dung, không phải phím điều hướng.
Ctrl+C không thoát khi bất kỳ tab nào chưa lưu hoặc Save còn đang chờ xác nhận;
Ctrl+S để lưu hoặc Ctrl+G để bỏ thay đổi.
Dấu `*` bên tên file là nội dung chưa lưu. Có thể tiếp tục gõ khi đang Save;
phản hồi Save chỉ xác nhận snapshot đã gửi, không xóa các chỉnh sửa mới hơn.
Dòng `tabs:` liệt kê các tab nền trong `[...]` và tab đang xem trong `>...<`;
dấu `*` trong ngoặc là tab chưa lưu. Enter mở file mới vào tab mới; mở lại file
đang mở ở tab khác sẽ chuyển tới tab đó thay vì mở trùng. Không chuyển tab khi
Save đang bay để giữ đúng mapping xác nhận.

## Phạm vi hiện tại

- Nhiều tab, mỗi tab giữ document/input/undo-redo riêng; text UTF-8 tối đa 1 MiB.
  Binary và ký tự điều khiển terminal
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
- LSP cho file `.py` khi server chạy với `--lsp-python` (đường dẫn tới
  language server, ví dụ `pylsp`); file khác báo "no language server for this
  file type": hover (`Ctrl+K`), problems tự động, completion (`Ctrl+J`, tối đa
  20 gợi ý), signature help (tự động khi gõ `(` `,` hoặc `Ctrl+O`, tối đa
  5 overload), rename (`Ctrl+R`, áp dụng server-side cho mọi file trong
  workspace, tự tải lại buffer sạch), code action quickfix (`Ctrl+X`,
  áp dụng server-side, tối đa 8 hiển thị). Chưa có refactor phức tạp
  (extract, move) hay source action.
- Chưa có syntax highlighting, tạo/xóa file hay clipboard hệ điều hành.

Sau lỗi kết nối, chưa có tự reconnect; giữ/copy nội dung chưa lưu trước khi đóng client.
