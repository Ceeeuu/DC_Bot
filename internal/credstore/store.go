package credstore

import "sync"

// Credentials 儲存使用者的 TKU 帳密。
type Credentials struct {
	Username string
	Password string
}

// Store 介面，方便未來替換為 DB 實作。
type Store interface {
	Set(userID string, creds Credentials)
	Get(userID string) (Credentials, bool)
	Delete(userID string)
}

// Default 是全域預設的 in-memory store 實例。
var Default Store = NewMemoryStore()

// MemoryStore 是 Store 的 in-memory 實作。
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]Credentials
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{data: make(map[string]Credentials)}
}

func (s *MemoryStore) Set(userID string, creds Credentials) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[userID] = creds
}

func (s *MemoryStore) Get(userID string) (Credentials, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.data[userID]
	return c, ok
}

func (s *MemoryStore) Delete(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, userID)
}
