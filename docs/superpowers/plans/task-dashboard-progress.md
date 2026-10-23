# Task dashboard — execution ledger

Scope: approved plan tasks 2/3/10, read-only workspace task history, agent
statuses, bounded task event replay and mobile TUI navigation.

- [x] Workspace-scoped task pagination and bounded snapshot/event queries.
- [x] Server protocol endpoints with identity validation.
- [x] Dashboard/detail views, refresh and reconnect history restoration.
- [x] Integration tests, race checks, review and docs.

Ruling: use SQLite task rowid as a keyset pagination cursor (not timestamps)
because existing rows predate timestamp persistence. Cost: task creation times
remain unavailable; don't display fabricated timestamps.

Ruling: refresh task details explicitly, with bounded incremental event pages.
Continuous push subscriptions and automatic reconnect remain future work;
avoid background polling that would compete with editing on a slow SSH link.

Implementation notes:

- `tasks.list` returns workspace-scoped pages of at most 50 tasks. `l` loads the
  next page; `r` refreshes from the newest page.
- `tasks.snapshot` validates workspace and task identity, returns agents and
  incremental events after `AfterSequence`, and caps replay at 50 events and
  2 MiB of payload.
- The TUI restores task history after workspace open, opens task detail with
  Enter, refreshes logs incrementally with `r`, and sanitizes output before
  rendering so terminal control sequences cannot escape into the terminal.
- This is read-only dashboard functionality. Starting queued tasks from this
  view, live subscriptions, automatic reconnect and agent controls remain
  future work.

Verification completed:

```text
go test ./... -count=1
go test -race ./internal/store ./internal/server ./internal/ui
go vet ./...
git diff --check
```
