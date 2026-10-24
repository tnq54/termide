# Standalone Android test build

User priority: an installable APK for testing without Termux.

Implementation choices: existing Java Android app, bundled xterm.js WebView
terminal, embedded JSch SSH client, Go TUI on the VPS via `termide --local`.
This is an online SSH client, not an offline on-device IDE. No runtime CDN.

Screen: compact connection form → full-height terminal and touch key row.
Use slate blue #243B53 for connection controls, paper #F0F4F8 for form fields,
navy #102A43 for terminal, light ink #D9E2EC, amber #F0B429 for active Ctrl.
System sans for forms; monospace terminal. Left-aligned, no dashboard cards;
terminal gets the space on portrait screens, including with keyboard open.

- [x] Local Go transport and CLI mode; focused tests.
- [x] Embedded SSH, host fingerprint trust, password/private-key auth.
- [x] Bundled terminal, IME/touch keys, resize, offline demo.
- [x] Build signed debug APK and Linux VPS binaries.
- [x] Verify package, tests and document exact install/use steps and limits.

Credentials stay in memory; only connection details and accepted host keys
are stored privately. Backups disabled. New host keys require confirmation;
changed keys rejected. Disconnect when the activity is destroyed. Background
session persistence and auto-reconnect are outside this test build.

Verified: `go test ./...`, `go test -race` (lsp/server/client), `go vet`,
`git diff --check`, Gradle `:app:testDebugUnitTest` incl. real loopback sshd
(key auth, trust prompt, changed-key rejection, real `termide --local`
dashboard open/exit, PTY resize, Unicode), Playwright terminal spec
(offline demo + SSH form/UTF-8/Ctrl), fresh `assembleDebug`, `aapt dump
badging`, `apksigner verify`, Linux amd64 `termide`/`termide-server` builds.
