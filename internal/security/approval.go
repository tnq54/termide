package security

import (
	"fmt"
	"sync"

	"termide/internal/domain"
)

type ApprovalManager struct {
	mu sync.RWMutex
	pending map[string]domain.ApprovalRequest
}

func NewApprovalManager() *ApprovalManager { return &ApprovalManager{pending: make(map[string]domain.ApprovalRequest)} }
func (m *ApprovalManager) Request(request domain.ApprovalRequest) error {
	if request.ID == "" || request.TaskID == "" || request.AgentID == "" || request.Command == "" { return fmt.Errorf("approval request identity and command are required") }
	m.mu.Lock(); defer m.mu.Unlock()
	m.pending[request.ID] = request
	return nil
}
func (m *ApprovalManager) Pending(id string) (domain.ApprovalRequest, bool) { m.mu.RLock(); defer m.mu.RUnlock(); r, ok := m.pending[id]; return r, ok }
func (m *ApprovalManager) ApproveOnce(id string) error { return m.resolve(id) }
func (m *ApprovalManager) Deny(id string) error { return m.resolve(id) }
func (m *ApprovalManager) resolve(id string) error { m.mu.Lock(); defer m.mu.Unlock(); if _, ok := m.pending[id]; !ok { return fmt.Errorf("approval request %q not found", id) }; delete(m.pending, id); return nil }
