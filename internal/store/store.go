package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "modernc.org/sqlite"
	"termide/internal/domain"
)

//go:embed migrations/001_initial.sql
var migrations embed.FS

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	// SQLite connection pragmas apply per connection. Serialize writes and keep
	// foreign-key enforcement active for every operation.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		db.Close()
		return nil, err
	}
	migration, err := migrations.ReadFile("migrations/001_initial.sql")
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(string(migration)); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) GetWorkspace(ctx context.Context, id string) (domain.Workspace, error) {
	var w domain.Workspace
	err := s.db.QueryRowContext(ctx, `SELECT id,name,host,root FROM workspaces WHERE id=?`, id).Scan(&w.ID, &w.Name, &w.Host, &w.Root)
	return w, err
}

func (s *Store) CreateWorkspace(ctx context.Context, w domain.Workspace) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO workspaces(id,name,host,root) VALUES(?,?,?,?)`, w.ID, w.Name, w.Host, w.Root)
	return err
}

func (s *Store) CreateTask(ctx context.Context, t domain.Task) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO tasks(id,workspace_id,description,status) VALUES(?,?,?,?)`, t.ID, t.WorkspaceID, t.Description, t.Status)
	return err
}

func (s *Store) UpdateTaskStatus(ctx context.Context, id string, status domain.TaskStatus) error {
	_, err := s.db.ExecContext(ctx, `UPDATE tasks SET status=? WHERE id=?`, status, id)
	return err
}

func (s *Store) GetTask(ctx context.Context, id string) (domain.Task, error) {
	var t domain.Task
	err := s.db.QueryRowContext(ctx, `SELECT id,workspace_id,description,status FROM tasks WHERE id=?`, id).Scan(&t.ID, &t.WorkspaceID, &t.Description, &t.Status)
	return t, err
}

func (s *Store) AppendEvent(ctx context.Context, e domain.EventEnvelope) error {
	check := e
	if check.Sequence == 0 {
		check.Sequence = 1
	}
	if err := check.Validate(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO events(workspace_id,task_id,agent_id,type,payload) VALUES(?,?,?,?,?)`, e.WorkspaceID, e.TaskID, e.AgentID, e.Type, e.Payload)
	return err
}

func (s *Store) EventsAfter(ctx context.Context, workspaceID string, sequence uint64) ([]domain.EventEnvelope, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT workspace_id,task_id,agent_id,sequence,type,payload FROM events WHERE workspace_id=? AND sequence>? ORDER BY sequence`, workspaceID, sequence)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.EventEnvelope
	for rows.Next() {
		var e domain.EventEnvelope
		if err := rows.Scan(&e.WorkspaceID, &e.TaskID, &e.AgentID, &e.Sequence, &e.Type, &e.Payload); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) UpsertAgent(ctx context.Context, a domain.Agent) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agents(id,task_id,role,status,branch,worktree,summary) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET role=excluded.role,status=excluded.status,branch=excluded.branch,worktree=excluded.worktree,summary=excluded.summary`, a.ID, a.TaskID, a.Role, a.Status, a.Branch, a.Worktree, a.Summary)
	return err
}

func (s *Store) ListAgents(ctx context.Context, taskID string) ([]domain.Agent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,task_id,role,status,COALESCE(branch,''),COALESCE(worktree,''),COALESCE(summary,'') FROM agents WHERE task_id=? ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []domain.Agent
	for rows.Next() {
		var a domain.Agent
		if err := rows.Scan(&a.ID, &a.TaskID, &a.Role, &a.Status, &a.Branch, &a.Worktree, &a.Summary); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (s *Store) String() string { return fmt.Sprintf("Store(%p)", s) }
