package session

import "sync"

type Store struct {
	mu    sync.RWMutex
	items map[string]any
}

func NewStore() *Store {
	return &Store{items: make(map[string]any)}
}

func (s *Store) Save(id string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[id] = value
}

func (s *Store) Load(id string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.items[id]
	return value, ok
}
