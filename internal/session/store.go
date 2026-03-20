package session

import "sync"

// SessionStore is a thread-safe map from channel ID to Session.
//
// Each Telegram channel (chat) gets its own independent Session.
// The store is safe for concurrent use by multiple goroutines — callers
// never need external locking.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[int64]*Session
}

// NewSessionStore creates an empty SessionStore ready for use.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[int64]*Session),
	}
}

// Get returns the Session for channelID, or (nil, false) if none exists.
// Safe for concurrent use.
func (s *SessionStore) Get(channelID int64) (*Session, bool) {
	s.mu.RLock()
	sess, ok := s.sessions[channelID]
	s.mu.RUnlock()
	return sess, ok
}

// GetOrCreate returns the existing Session for channelID, or creates and stores a new
// one if none exists.  workingDir is used only when creating a new Session.
//
// The double-checked locking pattern ensures only one Session is ever created per
// channel even under concurrent calls:
//  1. Fast path: RLock, lookup (covers the common case — session already exists).
//  2. Slow path: RUnlock, Lock, lookup again (covers the race where two goroutines
//     both miss the fast path simultaneously).
//
// Callers are responsible for starting the Worker goroutine after the first creation.
func (s *SessionStore) GetOrCreate(channelID int64, workingDir string) *Session {
	// Fast path.
	s.mu.RLock()
	sess, ok := s.sessions[channelID]
	s.mu.RUnlock()
	if ok {
		return sess
	}

	// Slow path: create under write lock.
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock.
	if sess, ok = s.sessions[channelID]; ok {
		return sess
	}

	sess = NewSession(workingDir)
	s.sessions[channelID] = sess
	return sess
}

// Remove deletes the Session for channelID from the store.
// It does not stop or clean up the Session — callers must handle that separately.
func (s *SessionStore) Remove(channelID int64) {
	s.mu.Lock()
	delete(s.sessions, channelID)
	s.mu.Unlock()
}

// All returns a shallow copy of the sessions map.
// Safe for iteration without holding the store lock.
func (s *SessionStore) All() map[int64]*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int64]*Session, len(s.sessions))
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
