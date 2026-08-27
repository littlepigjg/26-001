package store

import (
	"config-center/model"
)

// RawSnapshot returns a defensive copy of the current short-URL group. The
// returned map is safe to use after the call without holding the store lock.
// It takes an RLock and reads the current snapshot pointer exactly once, so
// it never races with concurrent writers and never observes a half-built map.
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	result := make(map[string]model.ShortURL)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.group == nil {
		return result
	}

	group := s.group.Load().(map[string]*model.ShortURL)
	for code, u := range group {
		if u != nil {
			result[code] = *u
		}
	}

	return result
}
