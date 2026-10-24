# LSP transport correctness

- [x] Inspect framing and connection ownership.
- [x] Reproduce coalesced-frame loss and concurrent-write corruption.
- [x] Keep one buffered reader per connection and serialize frame writes.
- [x] Run regression tests, full suite, race detector and vet.

Verified: `go test ./... -count=1 -timeout 300s`,
`go test -race ./internal/lsp ./internal/server -count=1 -timeout 300s`,
`go vet ./...`, and `git diff --check`.

Both regression tests failed before the fix: the second coalesced notification
was lost, and concurrent frame writes produced invalid JSON. They pass after
the fix. Unsaved-buffer synchronization and UTF-16 positions remain separate
work; these transport fixes do not implement those features.
