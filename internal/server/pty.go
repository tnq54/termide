package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/creack/pty"
)

const (
	maxPTYBuffer   = 256 * 1024
	defaultPTYIdle = 30 * time.Minute
	maxPTYRows     = 500
	maxPTYCols     = 500
)

type ptySession struct {
	id        string
	workspace string
	cmd       *exec.Cmd
	ptmx      *os.File
	// mu is an RWMutex: readers/writers hold RLock across ptmx IO while
	// kill takes Lock, so Close can never race an in-flight Write onto a
	// recycled file descriptor.
	mu         sync.RWMutex
	buf        []byte
	base       uint64
	lastActive time.Time
	done       chan struct{}
	dead       bool
}

type PTYRegistry struct {
	mu       sync.Mutex
	sessions map[string]*ptySession
	idle     time.Duration
}

func NewPTYRegistry() *PTYRegistry {
	return &PTYRegistry{sessions: make(map[string]*ptySession), idle: defaultPTYIdle}
}

func clampSize(value, max int) int {
	if value < 1 {
		return 1
	}
	if value > max {
		return max
	}
	return value
}

// Open starts an interactive shell in dir and returns its session ID. dir
// must be an absolute workspace directory; the session is bound to
// workspaceID and every later operation re-checks that binding, so one
// workspace can never drive another workspace's shell by guessing IDs.
func (r *PTYRegistry) Open(ctx context.Context, workspaceID, dir string, rows, cols int) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if workspaceID == "" {
		return "", fmt.Errorf("workspace ID is required")
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("pty directory must be absolute")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("pty directory is not a directory")
	}
	ws := &pty.Winsize{Rows: uint16(clampSize(rows, maxPTYRows)), Cols: uint16(clampSize(cols, maxPTYCols))}
	cmd := exec.Command("sh")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptmx, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return "", err
	}
	r.mu.Lock()
	r.sweepLocked()
	id := fmt.Sprintf("pty-%d", time.Now().UnixNano())
	session := &ptySession{id: id, workspace: workspaceID, cmd: cmd, ptmx: ptmx, lastActive: time.Now(), done: make(chan struct{})}
	r.sessions[id] = session
	r.mu.Unlock()
	go r.drain(session)
	return id, nil
}

func (r *PTYRegistry) drain(session *ptySession) {
	defer close(session.done)
	chunk := make([]byte, 4096)
	for {
		n, err := session.ptmx.Read(chunk)
		session.mu.Lock()
		session.lastActive = time.Now()
		if n > 0 {
			session.buf = append(session.buf, chunk[:n]...)
			if len(session.buf) > maxPTYBuffer {
				overflow := len(session.buf) - maxPTYBuffer
				session.buf = append([]byte(nil), session.buf[overflow:]...)
				session.base += uint64(overflow)
			}
		}
		session.mu.Unlock()
		if err != nil {
			return
		}
	}
}

func (r *PTYRegistry) lookup(workspaceID, id string) (*ptySession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[id]
	if !ok {
		return nil, fmt.Errorf("pty session not found")
	}
	if session.workspace != workspaceID {
		return nil, fmt.Errorf("pty session belongs to another workspace")
	}
	return session, nil
}

func (r *PTYRegistry) sweep() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
}

func (r *PTYRegistry) Write(ctx context.Context, workspaceID, id string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.sweep()
	session, err := r.lookup(workspaceID, id)
	if err != nil {
		return err
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	if session.dead {
		return fmt.Errorf("pty session not found")
	}
	_, err = session.ptmx.Write(data)
	return err
}

// Read returns output after the absolute offset `after` plus the new end
// offset. Offsets older than the retained window clamp to the oldest kept
// byte.
func (r *PTYRegistry) Read(ctx context.Context, workspaceID, id string, after uint64) ([]byte, uint64, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	r.sweep()
	session, err := r.lookup(workspaceID, id)
	if err != nil {
		return nil, 0, err
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	if session.dead {
		return nil, 0, fmt.Errorf("pty session not found")
	}
	session.lastActive = time.Now()
	start := after
	if start < session.base {
		start = session.base
	}
	if start > session.base+uint64(len(session.buf)) {
		start = session.base + uint64(len(session.buf))
	}
	out := append([]byte(nil), session.buf[start-session.base:]...)
	return out, session.base + uint64(len(session.buf)), nil
}

func (r *PTYRegistry) Resize(ctx context.Context, workspaceID, id string, rows, cols int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	session, err := r.lookup(workspaceID, id)
	if err != nil {
		return err
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	if session.dead {
		return fmt.Errorf("pty session not found")
	}
	return pty.Setsize(session.ptmx, &pty.Winsize{Rows: uint16(clampSize(rows, maxPTYRows)), Cols: uint16(clampSize(cols, maxPTYCols))})
}

func (r *PTYRegistry) Close(workspaceID, id string) error {
	r.mu.Lock()
	session, ok := r.sessions[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("pty session not found")
	}
	if session.workspace != workspaceID {
		r.mu.Unlock()
		return fmt.Errorf("pty session belongs to another workspace")
	}
	delete(r.sessions, id)
	r.mu.Unlock()
	session.kill()
	return nil
}

func (r *PTYRegistry) CloseAll() {
	r.mu.Lock()
	sessions := make([]*ptySession, 0, len(r.sessions))
	for id, session := range r.sessions {
		sessions = append(sessions, session)
		delete(r.sessions, id)
	}
	r.mu.Unlock()
	for _, session := range sessions {
		session.kill()
	}
}

func (s *ptySession) kill() {
	// SIGKILL, not SIGHUP: shells vary in SIGHUP handling once the master
	// side closes, but SIGKILL always releases the drain and Wait. Lock
	// excludes in-flight Reads/Writes so no IO can race the close.
	s.mu.Lock()
	if s.dead {
		s.mu.Unlock()
		return
	}
	s.dead = true
	_ = s.cmd.Process.Kill()
	_ = s.ptmx.Close()
	s.mu.Unlock()
	<-s.done
	_ = s.cmd.Wait()
}

func (r *PTYRegistry) sweepLocked() {
	now := time.Now()
	for id, session := range r.sessions {
		session.mu.RLock()
		idle := now.Sub(session.lastActive)
		session.mu.RUnlock()
		if idle > r.idle {
			delete(r.sessions, id)
			go session.kill()
		}
	}
}
