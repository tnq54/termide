# LSP Unicode edit correctness

- [x] Trace returned LSP edit positions through applyTextEdits.
- [x] Reproduce UTF-16/rune mismatch and invalid surrogate boundaries.
- [x] Convert UTF-16 positions to rune offsets before applying edits.
- [x] Verify full suite, race checks and vet.

Scope: server-side edits shared by rename, quickfix, source action and refactor.
Outgoing cursor positions and unsaved-buffer synchronization are follow-up work.

Regression evidence: before the fix, renaming `name` in `😀name = 1` produced
`😀ntotal= 1`; replacing an emoji also deleted the next character. The tests
now pass for replacement/insertion, multiple edits, multiline CRLF, BMP and
combining marks, and rejection of split surrogate pairs and invalid offsets.

Verified with `go test ./... -count=1 -timeout 300s`,
`go test -race ./internal/server ./internal/lsp -count=1 -timeout 300s`,
`go vet ./...`, and `git diff --check`.
