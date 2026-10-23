CREATE TABLE IF NOT EXISTS workspaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    host TEXT NOT NULL,
    root TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id),
    description TEXT NOT NULL,
    status TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS events (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    type TEXT NOT NULL,
    payload BLOB
);
CREATE INDEX IF NOT EXISTS events_workspace_sequence ON events(workspace_id, sequence);
CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id),
    role TEXT NOT NULL,
    status TEXT NOT NULL,
    branch TEXT,
    worktree TEXT,
    summary TEXT
);
