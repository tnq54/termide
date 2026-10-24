CREATE TABLE IF NOT EXISTS task_runs (
    run_id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id),
    plan_json BLOB NOT NULL,
    owner TEXT NOT NULL,
    lease_until INTEGER NOT NULL,
    attempt INTEGER NOT NULL,
    cancel_requested INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS task_runs_lease ON task_runs(status, lease_until);
