package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Service) LoadLocaleMessages() map[string]map[string]string {
	localeDir := filepath.Join(filepath.Dir(s.cfgPath), "locales")
	entries, err := os.ReadDir(localeDir)
	if err != nil {
		return map[string]map[string]string{}
	}

	catalog := make(map[string]map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}

		locale := strings.TrimSpace(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())))
		if locale == "" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(localeDir, entry.Name()))
		if err != nil {
			s.logf("locale load failed for %s: %v", entry.Name(), err)
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			s.logf("locale parse failed for %s: %v", entry.Name(), err)
			continue
		}

		flat := map[string]string{}
		flattenLocaleMap("", raw, flat)
		if len(flat) == 0 {
			continue
		}
		catalog[locale] = flat
	}

	return catalog
}

func flattenLocaleMap(prefix string, value any, out map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			nextKey := key
			if prefix != "" {
				nextKey = prefix + "." + key
			}
			flattenLocaleMap(nextKey, nested, out)
		}
	case string:
		if prefix != "" {
			out[prefix] = typed
		}
	case bool:
		if prefix != "" {
			if typed {
				out[prefix] = "true"
			} else {
				out[prefix] = "false"
			}
		}
	case float64:
		if prefix != "" {
			out[prefix] = strconv.FormatFloat(typed, 'f', -1, 64)
		}
	}
}
