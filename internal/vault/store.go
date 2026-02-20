package vault

import "sync"

// Info holds the vault key material for a session.
type Info struct {
	ECDSAPublicKey string
	EdDSAPublicKey string
	ChainCode      string
}

// Store is a concurrency-safe, session-keyed vault state store.
type Store struct {
	mu     sync.RWMutex
	vaults map[string]Info
}

func NewStore() *Store {
	return &Store{
		vaults: make(map[string]Info),
	}
}

func (s *Store) Set(sessionID string, info Info) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vaults[sessionID] = info
}

func (s *Store) Get(sessionID string) (Info, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.vaults[sessionID]
	return info, ok
}

func (s *Store) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.vaults, sessionID)
}
