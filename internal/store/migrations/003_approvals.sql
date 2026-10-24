CREATE TABLE IF NOT EXISTS approvals (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id),
    agent_id TEXT NOT NULL,
    command TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    resolved_reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS approvals_task_status ON approvals(task_id, status);
