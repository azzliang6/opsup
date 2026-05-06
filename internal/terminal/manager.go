package terminal

import (
	"sync"

	"github.com/azzliang6/opsup/internal/sshclient"
)

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*sshclient.Session
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*sshclient.Session),
	}
}

func (m *Manager) Register(id string, sess *sshclient.Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[id] = sess
}

func (m *Manager) Unregister(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		s.Close()
		delete(m.sessions, id)
	}
}

func (m *Manager) CloseAllForServer(serverID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		_ = s
		delete(m.sessions, id)
	}
}
