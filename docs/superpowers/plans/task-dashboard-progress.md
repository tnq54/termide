# Task dashboard — execution ledger

## Session task cancellation

- [x] RED→GREEN: `cancel` across Remote/Serve stops an owned runner, preserves
  the connection, and persists cancelled task/agent status.
- [x] Reject cross-workspace cancellation without stopping the runner; reject
  completed and unowned jobs without mutating stored status.
- [x] RED→GREEN: dashboard `c` sends cancellation once while busy and reloads
  snapshot without inventing a terminal status. Failed requests retain state.
- [x] Full suite, server/UI/agent race tests and vet passed.

Cancel acknowledgement is `tasks.cancel_requested`, not a fabricated journal
event. Runtime owns final state persistence. Cancellation remains session-scoped;
the process-tree limitation documented above still applies.

## Background execution (current behavior)

- [x] RED→GREEN integration: Start acknowledges before a blocking runner ends;
  duplicate Start is rejected while snapshot and file requests still succeed.
- [x] Stream shutdown cancels owned jobs and waits for final task persistence.
- [x] Runtime Prepare claims queued tasks synchronously; execution runs once
  asynchronously using a copied plan. Existing synchronous Start uses the same path.
- [x] Full suite, agent/server/UI/store race tests and vet passed.
- [x] Independent review: background finalization errors were discarded.
  Fixed with a failing closed-store regression: Close and Serve now return
  aggregated task errors after waiting for jobs. Full suite/race/vet passed.

Ruling: scope background ownership to the current stdio session, cancelling
on disconnect — avoids orphan jobs while a persistent daemon is absent — cost:
jobs do not survive reconnect. Historical synchronous limitations below are
superseded by this section. Process-tree cancellation remains unverified.

## Follow-up: viewport and start correctness

- [x] RED→GREEN: offline refresh never schedules IO or locks the dashboard.
- [x] RED→GREEN: selected task and last detail log remain reachable with scrolling.
- [x] RED→GREEN: cell-width clipping and heights 1–12 preserve viewport bounds.
- [x] RED→GREEN: start sends task description and unique agent IDs.
- [x] RED→GREEN: Remote → Serve → Start → snapshot reload reports completion/logs.
- [x] Full Go suite, UI/server race tests and vet passed after changes.

Correction to earlier completion claims: synchronous Start still monopolizes
the transport; the 24-hour timeout is not a detached supervisor. Durable task
supervision needs further work.

## Follow-up: response validation and pagination

- [x] RED→GREEN: reject foreign snapshot/task/agent/event identities,
  replayed sequences, inconsistent event cursors, missing/foreign Start events.
- [x] RED→GREEN: failed refresh preserves the existing page cursor.
- [x] Real server integration: 56 tasks paginate without duplicates; refresh
  replaces the list rather than appending another first page.

Page results carry the request's `before` cursor, so response application no
longer infers append/replace from mutable UI state. Start events are validated
as acknowledgments; their current synthetic sequence is not used for replay.

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
- Dashboard detail now supports `s` for queued tasks. The server accepts a
  JSON agent plan on `start_task`; the production stdio binary wires
  `OpenCodeRunner` using `--opencode`. The request remains synchronous until a
  detached supervisor is added. Cancel, live subscriptions and automatic
  reconnect remain future work.

Verification completed:

```text
go test ./... -count=1
go test -race ./internal/store ./internal/server ./internal/ui
go vet ./...
git diff --check
```
