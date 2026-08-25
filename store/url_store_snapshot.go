package store

import (
	"config-center/model"
)

func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	result := make(map[string]model.ShortURL)

	if s.configs == nil {
		return result
	}

	av, ok := s.configs[s.groupName]
	if !ok {
		return result
	}

	group := av.Load()
	if group == nil {
		return result
	}

	urlMap, ok := group.(map[string]*model.ShortURL)
	if !ok {
		return result
	}

	for code, u := range urlMap {
		if u != nil {
			result[code] = *u
		}
	}

	return result
}
