package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const DefaultAsteriskName = "HI3LoaderV1"

type SavedAccount struct {
	Account       string `json:"account"`
	Password      string `json:"-"`
	UID           int64  `json:"uid,omitempty"`
	AccessKey     string `json:"-"`
	UName         string `json:"uname,omitempty"`
	LastLoginSucc bool   `json:"last_login_succ,omitempty"`
}

type DeviceProfile struct {
	AndroidID       string `json:"-"`
	MACAddress      string `json:"-"`
	IMEI            string `json:"-"`
	RuntimeUDID     string `json:"-"`
	UserProfileUDID string `json:"-"`
	CurBuvid        string `json:"-"`
}

type Config struct {
	CurrentAccount    string         `json:"current_account,omitempty"`
	SleepTime         int            `json:"sleep_time"`
	AutoClose         bool           `json:"auto_close"`
	GamePath          string         `json:"game_path,omitempty"`
	AsteriskName      string         `json:"asterisk_name,omitempty"`
	Accounts          []SavedAccount `json:"accounts,omitempty"`
	AutoWindowCapture bool           `json:"auto_window_capture"`
	AccountLogin      bool           `json:"account_login"`
	LoaderAPIBaseURL  string         `json:"loader_api_base_url,omitempty"`
	BackgroundImage   string         `json:"background_image,omitempty"`
	BackgroundOpacity float64        `json:"background_opacity"`
	PanelBlur         bool           `json:"panel_blur"`
	DeviceProfile     DeviceProfile  `json:"-"`
}

func Default() *Config {
	return &Config{
		SleepTime:         3,
		AsteriskName:      DefaultAsteriskName,
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

	cfg, err := decodeStoredConfig(data)
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
	if cfg.Normalize() {
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
	if err := AtomicWriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	clone := *c
	if len(c.Accounts) > 0 {
		clone.Accounts = append([]SavedAccount(nil), c.Accounts...)
	}
	clone.DeviceProfile = c.DeviceProfile
	clone.Normalize()
	return &clone
}

func (c *Config) Normalize() bool {
	if c == nil {
		return false
	}
	changed := false

	changed = normalizeStringField(&c.CurrentAccount) || changed
	changed = normalizeStringField(&c.GamePath) || changed
	changed = normalizeStringField(&c.LoaderAPIBaseURL) || changed
	changed = normalizeStringField(&c.BackgroundImage) || changed
	changed = normalizeStringField(&c.AsteriskName) || changed
	changed = normalizeDeviceProfile(&c.DeviceProfile) || changed
	if c.AsteriskName == "" {
		c.AsteriskName = DefaultAsteriskName
		changed = true
	}
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

	changed = normalizeSavedAccounts(c) || changed

	return changed
}

func normalizeSavedAccounts(c *Config) bool {
	if c == nil {
		return false
	}

	changed := false
	normalized := make([]SavedAccount, 0, len(c.Accounts))
	seen := make(map[string]struct{}, len(c.Accounts))

	appendIfValid := func(entry SavedAccount) {
		if !normalizeSavedAccount(&entry) {
			changed = true
			return
		}
		identity := savedAccountIdentity(entry.Account)
		if identity == "" {
			changed = true
			return
		}
		if _, exists := seen[identity]; exists {
			changed = true
			return
		}
		seen[identity] = struct{}{}
		normalized = append(normalized, entry)
	}

	for _, entry := range c.Accounts {
		appendIfValid(entry)
	}

	if c.CurrentAccount == "" && len(normalized) > 0 {
		c.CurrentAccount = normalized[0].Account
		changed = true
	}

	if len(normalized) == 0 {
		if len(c.Accounts) != 0 {
			c.Accounts = nil
			changed = true
		}
		c.CurrentAccount = ""
		return changed
	}

	if len(normalized) != len(c.Accounts) {
		c.Accounts = normalized
		changed = true
	}

	if len(c.Accounts) == len(normalized) {
		for idx := range normalized {
			if c.Accounts[idx] != normalized[idx] {
				c.Accounts = normalized
				changed = true
				break
			}
		}
	}

	if c.CurrentAccount != "" {
		if _, ok := c.FindSavedAccount(c.CurrentAccount); !ok && len(c.Accounts) > 0 {
			c.CurrentAccount = c.Accounts[0].Account
			changed = true
		}
	}

	return changed
}

func normalizeSavedAccount(entry *SavedAccount) bool {
	if entry == nil {
		return false
	}
	normalizeStringField(&entry.Account)
	normalizeStringField(&entry.Password)
	normalizeStringField(&entry.AccessKey)
	normalizeStringField(&entry.UName)
	if entry.Account == "" {
		return false
	}
	if entry.AccessKey == "" {
		entry.LastLoginSucc = false
	}
	return true
}

func savedAccountIdentity(account string) string {
	account = strings.TrimSpace(strings.ToLower(account))
	return account
}

func normalizeDeviceProfile(profile *DeviceProfile) bool {
	if profile == nil {
		return false
	}
	changed := false
	changed = normalizeStringField(&profile.AndroidID) || changed
	changed = normalizeStringField(&profile.MACAddress) || changed
	changed = normalizeStringField(&profile.IMEI) || changed
	changed = normalizeStringField(&profile.RuntimeUDID) || changed
	changed = normalizeStringField(&profile.UserProfileUDID) || changed
	changed = normalizeStringField(&profile.CurBuvid) || changed
	return changed
}

func (p DeviceProfile) IsComplete() bool {
	return strings.TrimSpace(p.AndroidID) != "" &&
		strings.TrimSpace(p.MACAddress) != "" &&
		strings.TrimSpace(p.IMEI) != "" &&
		strings.TrimSpace(p.RuntimeUDID) != "" &&
		strings.TrimSpace(p.UserProfileUDID) != "" &&
		strings.TrimSpace(p.CurBuvid) != ""
}

func (c *Config) FindSavedAccount(account string) (SavedAccount, bool) {
	if c == nil {
		return SavedAccount{}, false
	}
	identity := savedAccountIdentity(account)
	if identity == "" {
		return SavedAccount{}, false
	}
	for _, entry := range c.Accounts {
		if savedAccountIdentity(entry.Account) == identity {
			return entry, true
		}
	}
	return SavedAccount{}, false
}

func (c *Config) UpsertSavedAccount(entry SavedAccount) bool {
	if c == nil {
		return false
	}
	if !normalizeSavedAccount(&entry) {
		return false
	}
	identity := savedAccountIdentity(entry.Account)
	for idx := range c.Accounts {
		if savedAccountIdentity(c.Accounts[idx].Account) != identity {
			continue
		}
		if c.Accounts[idx] == entry {
			return false
		}
		c.Accounts[idx] = entry
		return true
	}
	c.Accounts = append(c.Accounts, entry)
	return true
}

func (c *Config) ApplySavedAccount(account string) bool {
	if c == nil {
		return false
	}
	entry, ok := c.FindSavedAccount(account)
	if !ok {
		return false
	}

	changed := false
	if c.CurrentAccount != entry.Account {
		c.CurrentAccount = entry.Account
		changed = true
	}
	if c.AccountLogin {
		c.AccountLogin = false
		changed = true
	}
	return changed
}

func (c *Config) CurrentSavedAccount() (SavedAccount, bool) {
	if c == nil {
		return SavedAccount{}, false
	}
	if entry, ok := c.FindSavedAccount(c.CurrentAccount); ok {
		return entry, true
	}
	if len(c.Accounts) == 0 {
		return SavedAccount{}, false
	}
	return c.Accounts[0], true
}

func (c *Config) ClearSavedAccountSession(account string) bool {
	if c == nil {
		return false
	}
	identity := savedAccountIdentity(account)
	if identity == "" {
		return false
	}
	for idx := range c.Accounts {
		entry := &c.Accounts[idx]
		if savedAccountIdentity(entry.Account) != identity {
			continue
		}
		changed := false
		if entry.AccessKey != "" {
			entry.AccessKey = ""
			changed = true
		}
		if entry.LastLoginSucc {
			entry.LastLoginSucc = false
			changed = true
		}
		return changed
	}
	return false
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

func BoolValue(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	case int64:
		return x != 0
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return n != 0
		}
		if f, err := x.Float64(); err == nil {
			return f != 0
		}
		return false
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		switch s {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off", "":
			return false
		default:
			return false
		}
	default:
		return false
	}
}

func IntValue(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return int(n)
		}
		if f, err := x.Float64(); err == nil {
			return int(f)
		}
		return 0
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(x))
		return n
	default:
		return 0
	}
}

func Float64Value(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	default:
		return 0
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
