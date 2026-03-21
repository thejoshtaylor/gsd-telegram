package session

import "sync"

// SessionStore is a thread-safe map from instance ID to Session.
//
// Each instance (identified by a string instance ID or project name) gets its
// own independent Session. The store is safe for concurrent use by multiple
// goroutines — callers never need external locking.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionStore creates an empty SessionStore ready for use.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
	}
}

// Get returns the Session for instanceID, or (nil, false) if none exists.
// Safe for concurrent use.
func (s *SessionStore) Get(instanceID string) (*Session, bool) {
	s.mu.RLock()
	sess, ok := s.sessions[instanceID]
	s.mu.RUnlock()
	return sess, ok
}

// GetOrCreate returns the existing Session for instanceID, or creates and stores a new
// one if none exists. workingDir is used only when creating a new Session.
//
// The double-checked locking pattern ensures only one Session is ever created per
// instance even under concurrent calls:
//  1. Fast path: RLock, lookup (covers the common case — session already exists).
//  2. Slow path: RUnlock, Lock, lookup again (covers the race where two goroutines
//     both miss the fast path simultaneously).
//
// Callers are responsible for starting the Worker goroutine after the first creation.
func (s *SessionStore) GetOrCreate(instanceID string, workingDir string) *Session {
	// Fast path.
	s.mu.RLock()
	sess, ok := s.sessions[instanceID]
	s.mu.RUnlock()
	if ok {
		return sess
	}

	// Slow path: create under write lock.
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock.
	if sess, ok = s.sessions[instanceID]; ok {
		return sess
	}

	sess = NewSession(workingDir)
	s.sessions[instanceID] = sess
	return sess
}

// Remove deletes the Session for instanceID from the store.
// It does not stop or clean up the Session — callers must handle that separately.
func (s *SessionStore) Remove(instanceID string) {
	s.mu.Lock()
	delete(s.sessions, instanceID)
	s.mu.Unlock()
}

// All returns a shallow copy of the sessions map.
// Safe for iteration without holding the store lock.
func (s *SessionStore) All() map[string]*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*Session, len(s.sessions))
	for k, v := range s.sessions {
		out[k] = v
	}
	return out
}

// Count returns the number of sessions currently in the store.
func (s *SessionStore) Count() int {
	s.mu.RLock()
	n := len(s.sessions)
	s.mu.RUnlock()
	return n
}
