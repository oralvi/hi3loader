package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type DispatchCacheEntry struct {
	Data          string `json:"data"`
	Source        string `json:"source,omitempty"`
	RawLen        int    `json:"raw_len,omitempty"`
	DecodedLen    int    `json:"decoded_len,omitempty"`
	DecodedSHA256 string `json:"decoded_sha256,omitempty"`
	SavedAt       string `json:"saved_at,omitempty"`
}

type Config struct {
	Account   string `json:"account"`
	Password  string `json:"password"`
	SleepTime int    `json:"sleep_time"`
	Ver       int    `json:"ver"`
	ClipCheck bool   `json:"clip_check"`
	AutoClose bool   `json:"auto_close"`
	GamePath  string `json:"game_path,omitempty"`
	UID       int64  `json:"uid"`
	AccessKey string `json:"access_key"`
	// Additional credentials requested
	HI3UID            string                        `json:"HI3UID,omitempty"`
	BILIHITOKEN       string                        `json:"BILIHITOKEN,omitempty"`
	LastLoginSucc     bool                          `json:"last_login_succ"`
	BHVer             string                        `json:"bh_ver"`
	BiliPkgVer        int                           `json:"bili_pkg_ver,omitempty"` // 新增，缓存上次提取 token 时的版本号
	UName             string                        `json:"uname"`
	AutoClip          bool                          `json:"auto_clip"`
	AccountLogin      bool                          `json:"account_login"`
	VersionAPI        string                        `json:"version_api,omitempty"`
	DispatchAPI       string                        `json:"dispatch_api,omitempty"`
	DispatchData      string                        `json:"dispatch_data,omitempty"`
	DispatchCache     map[string]DispatchCacheEntry `json:"dispatch_cache,omitempty"`
	BackgroundImage   string                        `json:"background_image,omitempty"`
	BackgroundOpacity float64                       `json:"background_opacity"`
	PanelBlur         bool                          `json:"panel_blur"`
}

func Default() *Config {
	return &Config{
		SleepTime:         3,
		Ver:               5,
		BHVer:             "7.8.0",
		BackgroundOpacity: 0.35,
		PanelBlur:         true,
	}
}

func LoadOrCreate(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := Default()
		cfg.Normalize()
		if err := Save(path, cfg); err != nil {
			return nil, err
		}
		cfg.AccountLogin = false
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		cfg = Default()
		cfg.Normalize()
		if err := Save(path, cfg); err != nil {
			return nil, err
		}
		cfg.AccountLogin = false
		return cfg, nil
	}
	cfg.Ver = 5
	cfg.AccountLogin = false
	if cfg.Normalize() || needsUpgrade(data) {
		if err := Save(path, cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func Save(path string, cfg *Config) error {
	cfg.Normalize()
	cfg.Ver = 5
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	clone := *c
	clone.Normalize()
	return &clone
}

func (c *Config) Normalize() bool {
	if c == nil {
		return false
	}
	changed := false

	changed = normalizeStringField(&c.Account) || changed
	changed = normalizeStringField(&c.Password) || changed
	changed = normalizeStringField(&c.AccessKey) || changed
	changed = normalizeStringField(&c.UName) || changed
	changed = normalizeStringField(&c.GamePath) || changed
	changed = normalizeStringField(&c.BHVer) || changed
	changed = normalizeStringField(&c.VersionAPI) || changed
	changed = normalizeStringField(&c.DispatchAPI) || changed
	changed = normalizeStringField(&c.DispatchData) || changed
	changed = normalizeStringField(&c.BackgroundImage) || changed
	changed = normalizeStringField(&c.HI3UID) || changed
	changed = normalizeStringField(&c.BILIHITOKEN) || changed
	if c.SleepTime <= 0 {
		c.SleepTime = 3
		changed = true
	}
	if c.BackgroundOpacity < 0 {
		c.BackgroundOpacity = 0
		changed = true
	}
	if c.BackgroundOpacity > 1 {
		c.BackgroundOpacity = 1
		changed = true
	}
	if c.BHVer == "" {
		c.BHVer = "7.8.0"
		changed = true
	}
	if c.AccessKey == "" {
		if c.LastLoginSucc {
			changed = true
		}
		c.LastLoginSucc = false
		if c.UID == 0 {
			if c.UName != "" {
				changed = true
			}
			c.UName = ""
		}
	}

	normalizedCache := normalizeDispatchCache(c.DispatchCache)
	if !reflect.DeepEqual(c.DispatchCache, normalizedCache) {
		c.DispatchCache = normalizedCache
		changed = true
	}

	return changed
}

func StringValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return normalizeString(x)
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case json.Number:
		return x.String()
	default:
		return normalizeString(fmt.Sprintf("%v", x))
	}
}

func normalizeString(s string) string {
	switch strings.TrimSpace(s) {
	case "", "<nil>", "null", "<nil/>":
		return ""
	default:
		return s
	}
}

func normalizeStringField(target *string) bool {
	normalized := normalizeString(*target)
	if *target == normalized {
		return false
	}
	*target = normalized
	return true
}

func normalizeDispatchCache(cache map[string]DispatchCacheEntry) map[string]DispatchCacheEntry {
	if len(cache) == 0 {
		return nil
	}

	normalized := make(map[string]DispatchCacheEntry, len(cache))
	for key, entry := range cache {
		key = normalizeString(key)
		entry.Data = normalizeString(entry.Data)
		entry.Source = normalizeString(entry.Source)
		entry.DecodedSHA256 = normalizeString(entry.DecodedSHA256)
		entry.SavedAt = normalizeString(entry.SavedAt)
		if key == "" || entry.Data == "" {
			continue
		}
		normalized[key] = entry
	}

	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func Int64Value(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	default:
		return 0
	}
}

func needsUpgrade(raw []byte) bool {
	keys := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &keys); err != nil {
		return true
	}

	required := []string{
		"account_login",
		"background_opacity",
		"panel_blur",
	}
	for _, key := range required {
		if _, ok := keys[key]; !ok {
			return true
		}
	}
	return false
}
