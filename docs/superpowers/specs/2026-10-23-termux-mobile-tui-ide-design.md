# Thiết kế IDE TUI mobile-first cho Termux

## 1. Ý định và lý do

Xây dựng một IDE dạng TUI chạy trên Termux Android, giúp người dùng mở project trên VPS qua SSH, tạo công việc bằng mô tả tự nhiên và để nhiều AI agent phối hợp triển khai song song. Người dùng không phải thủ công mở Termux, SSH vào VPS rồi chạy từng công cụ CLI.

Android/Termux là lớp giao diện và điều khiển. VPS là nơi chứa source code, chạy OpenCode CLI, các công cụ phát triển, LSP và test.

## 2. Mục tiêu

- Chạy ổn định trên màn hình điện thoại, ưu tiên portrait và bàn phím terminal.
- Kết nối, lưu và chuyển đổi giữa nhiều VPS qua SSH.
- Mở project từ xa và quản lý workspace theo project/task.
- Có editor mã nguồn tích hợp trong TUI, không phụ thuộc vào `vim` hoặc `nvim`.
- Ưu tiên Python với syntax highlighting, file tree, tab/buffer, tìm kiếm, undo/redo và LSP đầy đủ.
- Tạo task bằng mô tả tự nhiên.
- Chạy nhiều agent song song thông qua OpenCode CLI trên VPS.
- Cô lập thay đổi bằng Git branch/worktree riêng cho từng agent.
- Hiển thị kế hoạch, trạng thái, lệnh, log, file thay đổi và diff theo thời gian thực.
- Tự động cho phép thao tác an toàn; yêu cầu người dùng phê duyệt thao tác nguy hiểm.
- Cho phép quay lại terminal trực tiếp và can thiệp thủ công.
- Agent tiếp tục chạy khi TUI tạm ngắt kết nối và khôi phục trạng thái khi kết nối lại.

## 3. Ngoài phạm vi phiên bản đầu

- Không xây model AI hoặc inference runtime riêng.
- Không xây hệ thống CI/CD, cloud build hay deployment platform hoàn chỉnh.
- Không hỗ trợ nhiều người dùng hoặc phân quyền team.
- Không cần tối ưu đồng thời cho desktop GUI.
- Không cam kết hỗ trợ đầy đủ ngôn ngữ ngoài Python trong bản đầu, nhưng kiến trúc LSP và syntax phải mở rộng được.
- Không chạy API key trong client nếu có thể cấu hình secret an toàn ở VPS.

## 4. Nguyên tắc trải nghiệm trên điện thoại

- Portrait là chế độ chính: mỗi thời điểm tập trung vào một panel lớn.
- Landscape là chế độ mở rộng: có thể hiển thị editor cùng task panel hoặc terminal.
- Không dựa vào chuột/touch; mọi chức năng quan trọng dùng được bằng bàn phím.
- Phím tắt ngắn, command palette và help overlay để giảm ghi nhớ.
- Output dài được rút gọn mặc định, có thể mở rộng, tìm kiếm và lọc.
- Thanh trạng thái cố định hiển thị project, VPS, branch, kết nối, agent và approval.
- Approval hiển thị lệnh, lý do, thư mục ảnh hưởng và nút cho phép/từ chối.
- Chuyển panel theo kiểu màn hình trượt, tránh chia quá nhiều ô nhỏ.
- TUI phải chịu được terminal nhỏ và resize khi xoay màn hình.

## 5. Luồng sử dụng chính

1. Người dùng mở IDE trong Termux.
2. Chọn hoặc thêm VPS bằng SSH key/config.
3. Chọn project và thư mục làm việc trên VPS.
4. IDE khởi tạo hoặc khôi phục workspace session.
5. Người dùng tạo task bằng mô tả tự nhiên.
6. Agent điều phối phân tích yêu cầu, tạo kế hoạch và chia thành các phần độc lập.
7. Các agent chuyên biệt chạy song song trong worktree/branch riêng.
8. TUI hiển thị trạng thái từng agent, lệnh đang chạy, log và thay đổi.
9. Khi gặp thao tác nguy hiểm, agent chuyển sang trạng thái chờ approval.
10. Người dùng cho phép, từ chối hoặc dừng agent.
11. Agent chạy test, review và báo cáo kết quả.
12. Agent điều phối tổng hợp thay đổi, báo xung đột và đề xuất merge.
13. Người dùng xem diff, chấp nhận merge hoặc tiếp tục task.

## 6. Các khu vực của IDE

### 6.1. Workspace/project

- Danh sách VPS và kết nối.
- Project gần đây và project yêu thích.
- Thư mục làm việc, branch và worktree hiện tại.
- Khôi phục session/task sau khi mất mạng.

### 6.2. Editor

- File tree.
- Mở nhiều file bằng tab/buffer.
- Syntax highlighting Python.
- Tìm file, tìm kiếm toàn project, tìm-thay thế.
- Undo/redo, copy/paste, chọn dòng/vùng.
- Hiển thị diff và marker thay đổi.
- LSP: autocomplete, diagnostics, go-to-definition, references, rename, formatting và code actions.
- Command palette cho thao tác ít dùng.

### 6.3. Task và agents

- Tạo task, sửa mô tả, xem kế hoạch.
- Danh sách agent với role, trạng thái, branch/worktree và tiến độ.
- Trạng thái: queued, planning, running, waiting approval, blocked, completed, failed, cancelled.
- Log theo agent và log tổng hợp.
- Lệnh đang chạy cùng working directory.
- Nút dừng, retry, pause/resume nếu backend hỗ trợ.
- Xem file đã thay đổi và diff theo agent.

### 6.4. Terminal

- Một hoặc nhiều terminal session qua SSH.
- Shell trực tiếp trên VPS.
- Chạy lệnh thủ công trong project/worktree đã chọn.
- Scrollback giới hạn có tìm kiếm.
- Không để terminal thủ công làm hỏng trạng thái task mà không cảnh báo.

### 6.5. Approval và review

- Hàng đợi approval tập trung.
- Phân loại lệnh an toàn/nguy hiểm dựa trên command, target path và tác động.
- Hiển thị command nguyên bản, agent yêu cầu, lý do và vùng ảnh hưởng.
- Allow once, allow for task, deny và cancel agent.
- Review diff trước merge.
- Phát hiện conflict và yêu cầu quyết định rõ ràng.

## 7. Kiến trúc kỹ thuật dự kiến

### 7.1. Thành phần trên Android/Termux

- TUI client chịu trách nhiệm rendering, input, navigation, editor và local state tối thiểu.
- SSH session manager tạo/kết nối các kênh SSH tới VPS.
- Remote workspace client giao tiếp với daemon/service trên VPS qua SSH tunnel hoặc protocol điều khiển.
- Secret store dùng cơ chế an toàn sẵn có của Termux/Android; không ghi API key vào log.

### 7.2. Thành phần trên VPS

- Workspace daemon duy trì session, task, agent, log, approval và event stream.
- OpenCode CLI là lớp gọi agent AI theo provider/API key bên thứ ba.
- Process supervisor bảo đảm agent tiếp tục chạy khi TUI ngắt kết nối.
- Git/worktree manager cô lập agent và xử lý merge.
- Terminal/session backend quản lý PTY và shell.
- Python tooling adapter phát hiện `venv`, `uv`, Poetry, pytest, ruff, mypy/pyright và lệnh project.
- LSP process manager khởi chạy và quản lý language server trong workspace.

### 7.3. Giao tiếp

Mọi event cần có task ID, agent ID, workspace ID và sequence number để client reconnect và tiếp tục từ event cuối đã nhận. Log/output phải stream theo chunk, có giới hạn lưu trữ và hỗ trợ truy vấn lại.

SSH là transport mặc định vì đã phù hợp với môi trường VPS. Thiết kế protocol điều khiển độc lập với UI để sau này có thể thêm client khác.

## 8. Multi-agent và cô lập thay đổi

- Agent điều phối tạo kế hoạch và các nhiệm vụ con.
- Nhiệm vụ độc lập chạy song song.
- Mỗi agent nhận branch/worktree riêng, tránh ghi đè trực tiếp.
- Agent chỉ được sửa phạm vi file được giao nếu policy task yêu cầu.
- Kết quả agent gồm summary, commands, tests, changed files và commit/patch reference.
- Agent review kiểm tra diff và test trước khi đề xuất merge.
- Merge tuần tự ở lớp điều phối; conflict không tự động ghi đè, phải báo người dùng hoặc agent được ủy quyền xử lý.

## 9. An toàn và kiểm soát

- API key chỉ nằm trong secret configuration ở VPS, không xuất hiện trong TUI log, command echo hoặc diff.
- Mặc định chặn hoặc yêu cầu approval cho xóa dữ liệu, thay đổi ngoài workspace, thay đổi quyền, cài package hệ thống, restart service, deploy, force push và lệnh shell có tác động rộng.
- Command classification không thay thế approval của người dùng đối với thao tác nhạy cảm.
- Mọi command agent chạy phải được audit log cùng timestamp, agent, task và working directory.
- Tách quyền process khi có thể; không chạy agent bằng root.
- Mất SSH không làm mất task; reconnect phải xác thực lại session và hiển thị event bị bỏ lỡ.

## 10. Python trong phiên bản đầu

- Ưu tiên Pyright hoặc `python-lsp-server` theo cấu hình project; builder có thể chọn implementation phù hợp sau khi kiểm tra dependency và hiệu năng.
- Tự nhận diện interpreter từ `.venv`, `venv`, `uv`, Poetry hoặc cấu hình project.
- Hỗ trợ pytest, ruff và mypy/pyright khi có trong project.
- LSP chạy ở VPS gần source code, không truyền toàn bộ source về Android.
- Cấu hình từng project có thể ghi đè mặc định.

## 11. Tiêu chí thành công

Một người dùng có thể từ Termux:

- Chọn VPS và mở project mà không phải tự nhập chuỗi SSH thủ công mỗi lần.
- Tạo task bằng một mô tả.
- Theo dõi ít nhất hai agent chạy song song.
- Mở file, chỉnh sửa, xem diagnostics và diff trong TUI.
- Nhận approval trước một lệnh nguy hiểm.
- Ngắt kết nối, kết nối lại và thấy task/log vẫn còn.
- Xem test result và merge thay đổi sau khi review.
- Làm tất cả việc trên terminal điện thoại ở portrait với số lần chuyển màn hình hợp lý.

## 12. Các quyết định để builder hoàn thiện

- Chọn ngôn ngữ và framework TUI/backend dựa trên khả năng triển khai trên Termux và VPS.
- Chọn protocol event/control cụ thể trên nền SSH.
- Chọn engine editor và LSP integration phù hợp, không phụ thuộc editor ngoài.
- Xác định schema lưu trữ session/task/log.
- Xác định policy phân loại command và cơ chế approval.
- Xác định phạm vi MVP có thể triển khai theo từng milestone mà không phá vỡ kiến trúc trên.
