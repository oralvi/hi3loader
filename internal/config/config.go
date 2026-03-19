package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const dispatchVersionSuffix = "_gf_android_bilibili"

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
	Password  string `json:"password,omitempty"`
	SleepTime int    `json:"sleep_time"`
	ClipCheck bool   `json:"clip_check"`
	AutoClose bool   `json:"auto_close"`
	GamePath  string `json:"game_path,omitempty"`
	UID       int64  `json:"uid"`
	AccessKey string `json:"access_key,omitempty"`
	// Additional credentials requested
	HI3UID                string  `json:"HI3UID,omitempty"`
	BILIHITOKEN           string  `json:"BILIHITOKEN,omitempty"`
	LastLoginSucc         bool    `json:"last_login_succ"`
	BHVer                 string  `json:"bh_ver"`
	BiliPkgVer            int     `json:"bili_pkg_ver,omitempty"` // Cached package version associated with the current BILIHITOKEN.
	UName                 string  `json:"uname"`
	AutoClip              bool    `json:"auto_clip"`
	AccountLogin          bool    `json:"account_login"`
	VersionAPI            string  `json:"version_api,omitempty"`
	DispatchAPI           string  `json:"dispatch_api,omitempty"`
	DispatchData          string  `json:"dispatch_data,omitempty"`
	DispatchVersion       string  `json:"dispatch_version,omitempty"`
	DispatchSource        string  `json:"dispatch_source,omitempty"`
	DispatchRawLen        int     `json:"dispatch_raw_len,omitempty"`
	DispatchDecodedLen    int     `json:"dispatch_decoded_len,omitempty"`
	DispatchDecodedSHA256 string  `json:"dispatch_decoded_sha256,omitempty"`
	DispatchSavedAt       string  `json:"dispatch_saved_at,omitempty"`
	BackgroundImage       string  `json:"background_image,omitempty"`
	BackgroundOpacity     float64 `json:"background_opacity"`
	PanelBlur             bool    `json:"panel_blur"`

	cryptoSalt string
}

func Default() *Config {
	return &Config{
		SleepTime:         3,
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

	cfg, storageChanged, err := decodeStoredConfig(data)
	if err != nil {
		if backupErr := backupCorruptConfig(path, data); backupErr != nil {
			return nil, fmt.Errorf("backup corrupt config: %w", backupErr)
		}
		if cfg == nil {
			cfg = Default()
		}
		cfg.Normalize()
		if err := Save(path, cfg); err != nil {
			return nil, err
		}
		cfg.AccountLogin = false
		return cfg, nil
	}
	cfg.AccountLogin = false
	if cfg.Normalize() || needsUpgrade(data) || storageChanged {
		if err := Save(path, cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func Save(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("save config: config is nil")
	}
	cfg.Normalize()
	stored, err := encodeStoredConfig(cfg)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := atomicWriteFile(path, data, 0o600); err != nil {
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
	changed = normalizeStringField(&c.DispatchVersion) || changed
	changed = normalizeStringField(&c.DispatchSource) || changed
	changed = normalizeStringField(&c.DispatchDecodedSHA256) || changed
	changed = normalizeStringField(&c.DispatchSavedAt) || changed
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

	changed = normalizeDispatchSnapshot(c) || changed

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

func NormalizeDispatchVersion(version string) string {
	version = normalizeString(version)
	if version == "" {
		return ""
	}
	if strings.Contains(version, "_gf_") {
		return version
	}
	return version + dispatchVersionSuffix
}

func (c *Config) DispatchSnapshot() (string, DispatchCacheEntry, bool) {
	if c == nil {
		return "", DispatchCacheEntry{}, false
	}

	entry := DispatchCacheEntry{
		Data:          normalizeString(c.DispatchData),
		Source:        normalizeString(c.DispatchSource),
		RawLen:        c.DispatchRawLen,
		DecodedLen:    c.DispatchDecodedLen,
		DecodedSHA256: normalizeString(c.DispatchDecodedSHA256),
		SavedAt:       normalizeString(c.DispatchSavedAt),
	}
	if entry.Data == "" {
		return "", DispatchCacheEntry{}, false
	}

	return NormalizeDispatchVersion(c.DispatchVersion), entry, true
}

func (c *Config) SetDispatchSnapshot(version string, entry DispatchCacheEntry) bool {
	if c == nil {
		return false
	}

	entry.Data = normalizeString(entry.Data)
	entry.Source = normalizeString(entry.Source)
	entry.DecodedSHA256 = normalizeString(entry.DecodedSHA256)
	entry.SavedAt = normalizeString(entry.SavedAt)
	version = NormalizeDispatchVersion(version)

	if entry.Data == "" {
		return c.ClearDispatchSnapshot()
	}

	changed := false
	if c.DispatchData != entry.Data {
		c.DispatchData = entry.Data
		changed = true
	}
	if c.DispatchVersion != version {
		c.DispatchVersion = version
		changed = true
	}
	if c.DispatchSource != entry.Source {
		c.DispatchSource = entry.Source
		changed = true
	}
	if c.DispatchRawLen != entry.RawLen {
		c.DispatchRawLen = entry.RawLen
		changed = true
	}
	if c.DispatchDecodedLen != entry.DecodedLen {
		c.DispatchDecodedLen = entry.DecodedLen
		changed = true
	}
	if c.DispatchDecodedSHA256 != entry.DecodedSHA256 {
		c.DispatchDecodedSHA256 = entry.DecodedSHA256
		changed = true
	}
	if c.DispatchSavedAt != entry.SavedAt {
		c.DispatchSavedAt = entry.SavedAt
		changed = true
	}
	return changed
}

func (c *Config) ClearDispatchSnapshot() bool {
	if c == nil {
		return false
	}
	changed := c.DispatchData != "" ||
		c.DispatchVersion != "" ||
		c.DispatchSource != "" ||
		c.DispatchRawLen != 0 ||
		c.DispatchDecodedLen != 0 ||
		c.DispatchDecodedSHA256 != "" ||
		c.DispatchSavedAt != ""

	c.DispatchData = ""
	c.DispatchVersion = ""
	c.DispatchSource = ""
	c.DispatchRawLen = 0
	c.DispatchDecodedLen = 0
	c.DispatchDecodedSHA256 = ""
	c.DispatchSavedAt = ""
	return changed
}

func normalizeDispatchSnapshot(c *Config) bool {
	if c == nil {
		return false
	}
	if normalizeString(c.DispatchData) == "" {
		return c.ClearDispatchSnapshot()
	}

	changed := false
	normalizedVersion := NormalizeDispatchVersion(c.DispatchVersion)
	if c.DispatchVersion != normalizedVersion {
		c.DispatchVersion = normalizedVersion
		changed = true
	}
	if c.DispatchRawLen < 0 {
		c.DispatchRawLen = 0
		changed = true
	}
	if c.DispatchDecodedLen < 0 {
		c.DispatchDecodedLen = 0
		changed = true
	}
	return changed
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
