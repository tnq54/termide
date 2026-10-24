# Server-Push + PTY Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Event push thật qua stream connection và terminal PTY tương tác.

**Architecture:** Stream connection dedicated cho push (`subscribe_stream` →
`replay` rồi `event`); Bridge bơm một chiều; PTY registry daemon-scoped với
CRUD qua request/response; UI stream có fallback, panel terminal dùng PTY poll.

**Tech Stack:** Go, creack/pty, framed JSON, Bubble Tea.

**Spec:** `docs/superpowers/specs/2026-10-25-push-pty-design.md`

## Global Constraints

- Không phá lockstep của interactive connection.
- Stream và PTY tôn trọng workspace identity.
- PTY output ring 256 KiB, idle reap 30 phút.
- Mọi test RED trước GREEN; full suite + race + vet + cross-build cuối mỗi task.

## Review Focus

- Đóng stream không giết interactive connection và ngược lại.
- Reconnect thấy PTY output cũ và event tiếp tục từ cursor.
- Race giữa poll/read và close session.
- Dialer lỗi → fallback sạch, không error-loop.

---

### Task 1: Event stream server-side

**Files:** `internal/protocol/messages.go`, `internal/server/stream.go`,
`internal/server/serve.go`, `internal/server/control.go`,
`internal/server/bridge.go`, tests.

- [x] RED→GREEN: subscribe_stream trả replay bù rồi push event mới; bridge bơm
  stream; unknown workspace bị từ chối.
- [x] Implement serveStream + hook vào Serve/ServeControl + bridge pump.
- [x] Tests + commit `feat: push workspace events over stream`.

### Task 2: Client stream API

**Files:** `internal/client/stream.go`, `internal/client/process.go`
(DialSSHStream), tests.

- [x] RED→GREEN: SubscribeStream nhận replay + event, từ chối sai identity, dừng
  khi ctx hủy mà không ảnh hưởng connection khác.
- [x] Implement + commit `feat: subscribe to pushed events`.

### Task 3: PTY registry + server handlers

**Files:** `internal/server/pty.go`, protocol pty messages, handler wiring,
Supervisor registry share, tests (pty thật).

- [x] RED→GREEN: open/write/read/resize/close; reconnect đọc output cũ; idle reap;
  workspace lạ bị từ chối.
- [x] Thêm creack/pty vào go.mod (module đã có trong cache).
- [x] Tests + commit `feat: daemon-scoped PTY sessions`.

### Task 4: Dashboard stream feed

**Files:** `internal/ui/*` stream dialer, fallback, tests; `cmd/termide/main.go`
wiring.

- [x] RED→GREEN: có dialer → mở stream, event live append; dial lỗi → wait-loop;
  đóng stream khi rời detail.
- [x] Implement + commit `feat: live dashboard event stream`.

### Task 5: Interactive PTY terminal panel

**Files:** `internal/ui/terminal_panel.go` (viết lại), tests; `docs/terminal.md`.

- [x] RED→GREEN: vào panel mở PTY theo kích thước màn hình; gõ phím write
  trực tiếp; poll read append và tiếp tục; resize gửi theo WindowSize;
  Esc close và quay về dashboard.
- [x] Phát hiện và sửa bug thật: phím `t` chưa bao giờ vào được panel
  terminal (dashboardKey nuốt mất).
- [x] `run_command` giữ nguyên cho automation.
- [x] Implement + commit `feat: interactive PTY terminal`.

### Task 6: Final verification

- [ ] `go test ./...`, race, vet, diff-check, android/linux/windows builds.
- [ ] Review diff, docs, commit docs.
