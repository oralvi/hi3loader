package config

import "sort"

func applyLegacyDispatchCache(cfg *Config, legacy map[string]DispatchCacheEntry) {
	normalized := normalizeDispatchCache(legacy)
	if len(normalized) == 0 || cfg == nil {
		return
	}

	if normalizeString(cfg.DispatchData) != "" {
		if cfg.DispatchVersion != "" && cfg.DispatchSource != "" && cfg.DispatchSavedAt != "" {
			return
		}
		if key, entry, ok := matchLegacyDispatchEntry(cfg.DispatchData, cfg.BHVer, normalized); ok {
			_, currentEntry, _ := cfg.DispatchSnapshot()
			cfg.SetDispatchSnapshot(key, mergeDispatchEntry(currentEntry, entry))
		}
		return
	}

	key, entry, ok := selectLegacyDispatchEntry(cfg.BHVer, normalized)
	if !ok {
		return
	}
	cfg.SetDispatchSnapshot(key, entry)
}

func mergeDispatchEntry(current, fallback DispatchCacheEntry) DispatchCacheEntry {
	if current.Data == "" {
		current.Data = fallback.Data
	}
	if current.Source == "" {
		current.Source = fallback.Source
	}
	if current.RawLen == 0 {
		current.RawLen = fallback.RawLen
	}
	if current.DecodedLen == 0 {
		current.DecodedLen = fallback.DecodedLen
	}
	if current.DecodedSHA256 == "" {
		current.DecodedSHA256 = fallback.DecodedSHA256
	}
	if current.SavedAt == "" {
		current.SavedAt = fallback.SavedAt
	}
	return current
}

func matchLegacyDispatchEntry(data, bhVer string, legacy map[string]DispatchCacheEntry) (string, DispatchCacheEntry, bool) {
	targetVersion := NormalizeDispatchVersion(bhVer)
	if targetVersion != "" {
		if entry, ok := legacy[targetVersion]; ok && normalizeString(entry.Data) == normalizeString(data) {
			return targetVersion, entry, true
		}
	}

	for key, entry := range legacy {
		if normalizeString(entry.Data) == normalizeString(data) {
			return key, entry, true
		}
	}
	return "", DispatchCacheEntry{}, false
}

func selectLegacyDispatchEntry(bhVer string, legacy map[string]DispatchCacheEntry) (string, DispatchCacheEntry, bool) {
	targetVersion := NormalizeDispatchVersion(bhVer)
	if targetVersion != "" {
		if entry, ok := legacy[targetVersion]; ok {
			return targetVersion, entry, true
		}
	}

	keys := make([]string, 0, len(legacy))
	for key := range legacy {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left := legacy[keys[i]]
		right := legacy[keys[j]]
		if left.SavedAt == right.SavedAt {
			return keys[i] < keys[j]
		}
		return left.SavedAt > right.SavedAt
	})
	if len(keys) == 0 {
		return "", DispatchCacheEntry{}, false
	}
	key := keys[0]
	return key, legacy[key], true
}
