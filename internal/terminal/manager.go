package terminal

import (
	"sync"

	"github.com/azzliang6/opsup/internal/sshclient"
)

type entry struct {
	serverID int64
	session  *sshclient.Session
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]entry
}

func NewManager() *Manager { return &Manager{sessions: make(map[string]entry)} }

func (m *Manager) Register(id string, sess *sshclient.Session, serverID ...int64) {
	var sid int64
	if len(serverID) > 0 {
		sid = serverID[0]
	}
	m.mu.Lock()
	old, exists := m.sessions[id]
	m.sessions[id] = entry{serverID: sid, session: sess}
	m.mu.Unlock()
	if exists && old.session != sess {
		old.session.Close()
	}
}

func (m *Manager) Unregister(id string) {
	m.mu.Lock()
	old, exists := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if exists {
		old.session.Close()
	}
}

func (m *Manager) CloseAllForServer(serverID int64) {
	var sessions []*sshclient.Session
	m.mu.Lock()
	for id, entry := range m.sessions {
		if entry.serverID == serverID {
			sessions = append(sessions, entry.session)
			delete(m.sessions, id)
		}
	}
	m.mu.Unlock()
	for _, session := range sessions {
		session.Close()
	}
}
