# LSP + APK Design

## LSP (bounded)

- Full JSON-RPC conn: request/response by ID + server notifications, over the
  language server's stdio. `Client` owns the process; tests use scripted pipes.
- Daemon-scoped `LSPManager` (supervisor-owned, like PTY): one server per
  workspace root, started on demand from `--lsp-python` binary. `didOpen`
  tracked per URI; hover sends didOpen first when unseen.
- Protocol: `lsp.hover {path,line,char}` → `lsp.hover_result {contents}`.
  Only `.py` files; other extensions get a clear error.
- TUI: Ctrl+K in editor requests hover at cursor (rune line/col; UTF-16
  limitation documented), popup overlay, Esc/movement dismisses.
- No diagnostics UI in this slice (documented future); server notifications
  are consumed and ignored.

## APK (launcher)

- Product is a Termux TUI; the shippable APK is a genuine launcher:
  `termide-launcher` sends `com.termux.app.RUN_COMMAND` to Termux:Task,
  opening Termux running the on-device `termide` binary with `--ssh/--project`.
- Built with real Android SDK (cmdline-tools + platform + build-tools) and
  Gradle wrapper; verified by `assembleDebug` + `aapt dump badging`.
- Requires the Termux + Termux:Task apps and the device-side binary;
  documented honestly. Full terminal-emulator APK is out of scope.
