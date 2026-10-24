# TermIDE Standalone APK (bản thử, không cần Termux)

APK Android thật, cài trực tiếp để test: terminal xterm.js đóng gói sẵn +
SSH client (JSch) kết nối tới VPS, chạy TUI Go (`termide --local`) trên VPS.
Không cần Termux/Termux:Task. Đây là bản thử online, không phải IDE offline.

## Yêu cầu

Trên điện thoại (Android 8.0+): cài APK debug trực tiếp, cho phép
“Install unknown apps” khi được hỏi. Không cần Termux.

Trên VPS (Linux amd64): `termide`, `termide-server` và supervisor daemon
đang chạy cho project cần mở. SSH mở port, đăng nhập bằng mật khẩu hoặc key.

## Build (đã verify trên Linux + JDK 17)

```sh
export ANDROID_HOME=/tmp/opencode/android-sdk
export PATH=/tmp/opencode/gradle-8.9/bin:$PATH
cd android
echo "sdk.dir=$ANDROID_HOME" > local.properties # file local, không commit
sh sync-terminal-assets.sh                       # npm ci + copy xterm vào assets
/tmp/opencode/gradle-8.9/bin/gradle :app:assembleDebug :app:testDebugUnitTest
```

Test SSH thật (sshd loopback cô lập, không đụng config máy):

```sh
TMPDIR=/tmp/opencode GRADLE=/tmp/opencode/gradle-8.9/bin/gradle sh android/test-ssh.sh
```

Test terminal WebView (cần Playwright Chromium đã tải sẵn):

```sh
cd terminal && PLAYWRIGHT_HOST_PLATFORM_OVERRIDE=ubuntu24.04-x64 npx playwright test
```

APK ra ở `app/build/outputs/apk/debug/app-debug.apk`. Kiểm tra:

```sh
$ANDROID_HOME/build-tools/34.0.0/aapt dump badging app-debug.apk | head -3
$ANDROID_HOME/build-tools/34.0.0/apksigner verify --print-certs app-debug.apk
```

## Cài và dùng để test

1. VPS: chép `termide-linux-amd64` → `~/bin/termide`,
   `termide-server-linux-amd64` → `~/bin/termide-server`, `chmod +x` cả hai
   (APK chạy `export PATH="$HOME/bin:$HOME/.local/bin:$PATH"` trước, nên để
   binary ở một trong hai chỗ). Lần kết nối đầu, `termide-server --stdio`
   tự khởi động supervisor daemon cho project nếu chưa chạy (log ở
   `termide.sock.log` cạnh socket) — không cần chạy `--daemon` tay.
   Muốn chạy tay vẫn được: `termide-server --daemon` trong thư mục project
   (mỗi project một socket/daemon).
2. Điện thoại: `adb install -r app-debug.apk`, mở app TermIDE.
3. Nhập Host/IP, Port, User; nhập mật khẩu hoặc Chọn SSH key (+ passphrase).
   Nhập đường dẫn project tuyệt đối (ví dụ `/home/ubuntu/project`).
4. Bấm “Mở TermIDE”. Lần đầu hiện fingerprint `SHA256:...` — đối chiếu với
   VPS (`ssh-keygen -l -f /etc/ssh/ssh_host_ed25519_key.pub`) rồi mới Tin cậy.
5. Dùng dãy phím Esc/Tab/Ctrl/mũi tên/^S/^C/Enter trên màn hình; nút A−/A+
   đổi cỡ chữ. Mất kết nối → kiểm tra mạng/đăng nhập/host key.
6. Không có VPS: bấm “Thử terminal không cần VPS” để test bàn phím, màu,
   cuộn, Ctrl+S ngay trên máy.

## Giới hạn trung thực (bản thử)

- APK debug ký bằng debug key — chỉ cài trực tiếp (`adb install`),
  không đưa lên Play.
- Mật khẩu/SSH key chỉ giữ trong RAM, không ghi xuống bộ nhớ máy; chỉ lưu
  host/port/user/đường dẫn và host key đã tin cậy (private, `allowBackup=false`).
  Key lạ hỏi mới tin; key đổi bị từ chối thẳng, không hỏi lại.
- Ngắt kết nối khi đóng app; không giữ session nền, không tự reconnect.
- Chưa test trên thiết bị thật ở đây: đã test SSH key/passphrase/trust/resize/
  Unicode/real-TUI qua sshd loopback, test WebView qua Playwright Chromium,
  và verify package/chữ ký; chưa có test chạm tay trên điện thoại.
- `local.properties` trỏ SDK máy build; đừng commit đường dẫn cá nhân khác.
