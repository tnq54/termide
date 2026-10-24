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

## Reconnect replay

- [x] RED→GREEN: `subscribe` validates workspace and returns `replay` events
  strictly after `AfterSequence`, preserving SQLite sequence order.
- [x] Client transport integration verifies cursor 1 receives events 2 and 3,
  while unrelated workspaces are excluded.
- [x] Typed client `Remote.Subscribe` validates response identity, event identity,
  strictly increasing sequence, and bounded event/payload size.
- [x] Server bounds replay to 10,000 events and 2 MiB before serialization.
- [x] Dashboard replays events after workspace reconnect when a task detail and
  cursor are already present, preserving task status and appending validated logs.
- [x] Durable supervisor: daemon owns task execution across SSH/stdio session
  loss; expired-lease recovery relaunches runs without duplicates; cancel from
  any session persists terminal state; single-daemon file lock enforced.
- [x] Daemon lifecycle integration: session close mid-run still reaches
  `completed`; reconnect observes final state via snapshot/replay.
- [ ] Live push remains future work.

## Long-poll subscribe

- [x] RED→GREEN: `subscribe` with `wait_ms` blocks until new events arrive,
  returns immediately when events exist, times out empty, and caps the wait
  at 30 seconds.
- [x] RED→GREEN: typed `Remote.SubscribeWait` sends the wait budget and
  reuses replay identity/sequence validation.
- [x] No background polling: the caller decides when to wait; full suite,
  race, vet and cross-builds passed.

## Dashboard live-wait loop

- [x] RED→GREEN: snapshot load schedules a 10s wait for non-terminal tasks
  without setting busy; new events append logs, advance the cursor and
  continue; errors and terminal tasks stop the loop.
- [x] RED→GREEN: stale/out-of-order wait results only apply events newer
  than the cursor; keys keep working while waiting.
- [x] RED→GREEN: end-to-end over real transport wakes on a late event.
- [x] Documented worst case: manual commands queue up to 10s behind an
  in-flight wait on the serial transport.

## Durable approval gate

- [x] RED→GREEN: approvals persist in SQLite; resolve is single-winner,
  duplicate and foreign resolutions rejected.
- [x] RED→GREEN: GateRunner auto-runs safe commands, blocks empty commands,
  and pauses dangerous specs until approval, resuming or failing without
  running the command.
- [x] RED→GREEN: daemon `approve` endpoint validates identity, emits
  approval events, and denies leftover pendings on cancel/stop.
- [x] Durable and session runners both gate specs; existing execution tests
  now use explicit safe commands under the new default-deny semantics.
- [x] RED→GREEN: dashboard lists pendings, `a`/`d` send decisions, foreign
  acks are rejected, and snapshot validation covers approval identity.
- [x] Full suite, race, vet, cross-builds passed.

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
