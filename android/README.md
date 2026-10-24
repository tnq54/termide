# TermIDE Launcher APK

APK thật nhưng mỏng: một launcher mở Termux qua Termux:Task (`RUN_COMMAND`)
để chạy binary `termide` đã cài trên thiết bị. Không phải terminal emulator;
toàn bộ TUI vẫn là binary Go chạy trong Termux.

## Yêu cầu trên điện thoại

- Termux (F-Droid) + Termux:Task.
- Binary `termide-android-arm64` chép vào
  `/data/data/com.termux/files/usr/bin/termide` và `chmod +x`.
- Cấp quyền RunCommand cho TermIDE khi được hỏi.

## Build (đã verify trên Linux + JDK 17)

```sh
export ANDROID_HOME=/path/to/android-sdk        # máy build này: /tmp/opencode/android-sdk
export PATH=/path/to/gradle-8.9/bin:$PATH
cd android
echo "sdk.dir=$ANDROID_HOME" > local.properties # file local, không commit
gradle assembleDebug -PtermideHost=vps -PtermideProject=/home/ubuntu/termux
```

APK ra ở `app/build/outputs/apk/debug/app-debug.apk`. Kiểm tra:

```sh
$ANDROID_HOME/build-tools/34.0.0/aapt dump badging app-debug.apk | head -3
$ANDROID_HOME/build-tools/34.0.0/apksigner verify --print-certs app-debug.apk
```

## Tùy biến host/project

- Build-time: `-PtermideHost=... -PtermideProject=...`.
- Runtime: app khác gọi launcher kèm extras `host`, `project` sẽ override
  giá trị build-in.

## Giới hạn trung thực

- Chưa test runtime trên thiết bị thật (không có device/emulator ở đây):
  contract intent theo tài liệu Termux:Task (`RUN_COMMAND`, service
  `com.termux.app.RunCommandService`, extras path/arguments/background/
  session_action). Nếu Termux:Task đổi contract, cập nhật `MainActivity`.
- Không có terminal emulator riêng; mất Termux là mất runtime.
- APK debug ký bằng debug key — chỉ cài trực tiếp (`adb install`),
  không đưa lên Play.
- `local.properties` trỏ SDK máy build; đừng commit đường dẫn cá nhân khác.
