package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

var allowedConfigKeys = map[string]struct{}{
	"current_account":     {},
	"sleep_time":          {},
	"auto_close":          {},
	"game_path":           {},
	"asterisk_name":       {},
	"accounts":            {},
	"auto_window_capture": {},
	"loader_api_base_url": {},
	"background_image":    {},
	"background_opacity":  {},
	"panel_blur":          {},
	"device_blob":         {},
}

type storedConfig struct {
	Config
}

type storedConfigJSON struct {
	CurrentAccount    string                   `json:"current_account,omitempty"`
	SleepTime         int                      `json:"sleep_time"`
	AutoClose         bool                     `json:"auto_close"`
	GamePath          string                   `json:"game_path,omitempty"`
	AsteriskName      string                   `json:"asterisk_name,omitempty"`
	Accounts          []storedSavedAccountJSON `json:"accounts,omitempty"`
	AutoWindowCapture bool                     `json:"auto_window_capture"`
	LoaderAPIBaseURL  string                   `json:"loader_api_base_url,omitempty"`
	BackgroundImage   string                   `json:"background_image,omitempty"`
	BackgroundOpacity float64                  `json:"background_opacity"`
	PanelBlur         bool                     `json:"panel_blur"`
	DeviceBlob        string                   `json:"device_blob,omitempty"`
}

type storedSavedAccountJSON struct {
	Account       string `json:"account"`
	SessionBlob   string `json:"session_blob,omitempty"`
	UID           int64  `json:"uid,omitempty"`
	UName         string `json:"uname,omitempty"`
	LastLoginSucc bool   `json:"last_login_succ,omitempty"`
}

type sessionSecretEnvelope struct {
	Password  string `json:"password,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
}

type deviceSecretEnvelope struct {
	AndroidID       string `json:"android_id,omitempty"`
	MACAddress      string `json:"mac,omitempty"`
	IMEI            string `json:"imei,omitempty"`
	RuntimeUDID     string `json:"runtime_udid,omitempty"`
	UserProfileUDID string `json:"user_profile_udid,omitempty"`
	CurBuvid        string `json:"cur_buvid,omitempty"`
}

func (s storedConfig) MarshalJSON() ([]byte, error) {
	accounts, err := encodeStoredSavedAccounts(s.Accounts)
	if err != nil {
		return nil, err
	}
	deviceBlob, err := encodeStoredDeviceProfile(s.DeviceProfile)
	if err != nil {
		return nil, err
	}
	payload := storedConfigJSON{
		CurrentAccount:    normalizeString(s.CurrentAccount),
		SleepTime:         s.SleepTime,
		AutoClose:         s.AutoClose,
		GamePath:          normalizeString(s.GamePath),
		AsteriskName:      normalizeString(s.AsteriskName),
		Accounts:          accounts,
		AutoWindowCapture: s.AutoWindowCapture,
		LoaderAPIBaseURL:  normalizeString(s.LoaderAPIBaseURL),
		BackgroundImage:   normalizeString(s.BackgroundImage),
		BackgroundOpacity: s.BackgroundOpacity,
		PanelBlur:         s.PanelBlur,
		DeviceBlob:        deviceBlob,
	}
	return json.Marshal(payload)
}

func encodeStoredConfig(cfg *Config) (*storedConfig, error) {
	clone := cfg.Clone()
	if clone == nil {
		return nil, fmt.Errorf("encode config: config is nil")
	}
	return &storedConfig{Config: *clone}, nil
}

func decodeStoredConfig(raw []byte) (*Config, error) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	for key := range rawMap {
		if _, ok := allowedConfigKeys[key]; !ok {
			return nil, fmt.Errorf("decode config: unsupported key %q", key)
		}
	}

	stored := storedConfigJSON{
		SleepTime:         Default().SleepTime,
		AsteriskName:      Default().AsteriskName,
		BackgroundOpacity: Default().BackgroundOpacity,
		PanelBlur:         Default().PanelBlur,
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	accounts, err := decodeStoredSavedAccounts(stored.Accounts)
	if err != nil {
		return nil, err
	}
	return &Config{
		CurrentAccount:    stored.CurrentAccount,
		SleepTime:         stored.SleepTime,
		AutoClose:         stored.AutoClose,
		GamePath:          stored.GamePath,
		AsteriskName:      stored.AsteriskName,
		Accounts:          accounts,
		AutoWindowCapture: stored.AutoWindowCapture,
		LoaderAPIBaseURL:  stored.LoaderAPIBaseURL,
		BackgroundImage:   stored.BackgroundImage,
		BackgroundOpacity: stored.BackgroundOpacity,
		PanelBlur:         stored.PanelBlur,
		DeviceProfile:     decodeStoredDeviceProfile(stored.DeviceBlob),
	}, nil
}

func backupCorruptConfig(path string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	backupPath := fmt.Sprintf("%s.corrupt-%s", path, time.Now().Format("20060102-150405"))
	return AtomicWriteFile(backupPath, data, 0o600)
}

func encodeStoredSavedAccounts(accounts []SavedAccount) ([]storedSavedAccountJSON, error) {
	if len(accounts) == 0 {
		return nil, nil
	}
	stored := make([]storedSavedAccountJSON, 0, len(accounts))
	for _, entry := range accounts {
		encoded, err := encodeStoredSavedAccount(entry)
		if err != nil {
			return nil, err
		}
		stored = append(stored, encoded)
	}
	return stored, nil
}

func encodeStoredSavedAccount(entry SavedAccount) (storedSavedAccountJSON, error) {
	stored := storedSavedAccountJSON{
		Account:       normalizeString(entry.Account),
		UID:           entry.UID,
		UName:         normalizeString(entry.UName),
		LastLoginSucc: entry.LastLoginSucc,
	}
	if strings.TrimSpace(entry.Password) == "" && strings.TrimSpace(entry.AccessKey) == "" {
		return stored, nil
	}
	raw, err := json.Marshal(sessionSecretEnvelope{
		Password:  normalizeString(entry.Password),
		AccessKey: normalizeString(entry.AccessKey),
	})
	if err != nil {
		return stored, fmt.Errorf("encode session blob: %w", err)
	}
	blob, err := protectSessionSecrets(raw)
	if err != nil {
		return stored, fmt.Errorf("protect session blob: %w", err)
	}
	stored.SessionBlob = blob
	return stored, nil
}

func decodeStoredSavedAccounts(accounts []storedSavedAccountJSON) ([]SavedAccount, error) {
	if len(accounts) == 0 {
		return nil, nil
	}
	decoded := make([]SavedAccount, 0, len(accounts))
	for _, entry := range accounts {
		account, err := decodeStoredSavedAccount(entry)
		if err != nil {
			return nil, err
		}
		decoded = append(decoded, account)
	}
	return decoded, nil
}

func decodeStoredSavedAccount(entry storedSavedAccountJSON) (SavedAccount, error) {
	decoded := SavedAccount{
		Account:       normalizeString(entry.Account),
		UID:           entry.UID,
		UName:         normalizeString(entry.UName),
		LastLoginSucc: entry.LastLoginSucc,
	}
	if strings.TrimSpace(entry.SessionBlob) == "" {
		return decoded, nil
	}
	raw, err := unprotectSessionSecrets(entry.SessionBlob)
	if err != nil {
		decoded.LastLoginSucc = false
		return decoded, nil
	}
	var secret sessionSecretEnvelope
	if err := json.Unmarshal(raw, &secret); err != nil {
		decoded.LastLoginSucc = false
		return decoded, nil
	}
	decoded.Password = normalizeString(secret.Password)
	decoded.AccessKey = normalizeString(secret.AccessKey)
	if decoded.AccessKey == "" {
		decoded.LastLoginSucc = false
	}
	return decoded, nil
}

func encodeStoredDeviceProfile(profile DeviceProfile) (string, error) {
	if !profile.IsComplete() {
		return "", nil
	}
	raw, err := json.Marshal(deviceSecretEnvelope{
		AndroidID:       normalizeString(profile.AndroidID),
		MACAddress:      normalizeString(profile.MACAddress),
		IMEI:            normalizeString(profile.IMEI),
		RuntimeUDID:     normalizeString(profile.RuntimeUDID),
		UserProfileUDID: normalizeString(profile.UserProfileUDID),
		CurBuvid:        normalizeString(profile.CurBuvid),
	})
	if err != nil {
		return "", fmt.Errorf("encode device blob: %w", err)
	}
	blob, err := protectSessionSecrets(raw)
	if err != nil {
		return "", fmt.Errorf("protect device blob: %w", err)
	}
	return blob, nil
}

func decodeStoredDeviceProfile(blob string) DeviceProfile {
	if strings.TrimSpace(blob) == "" {
		return DeviceProfile{}
	}
	raw, err := unprotectSessionSecrets(blob)
	if err != nil {
		return DeviceProfile{}
	}
	var secret deviceSecretEnvelope
	if err := json.Unmarshal(raw, &secret); err != nil {
		return DeviceProfile{}
	}
	return DeviceProfile{
		AndroidID:       normalizeString(secret.AndroidID),
		MACAddress:      normalizeString(secret.MACAddress),
		IMEI:            normalizeString(secret.IMEI),
		RuntimeUDID:     normalizeString(secret.RuntimeUDID),
		UserProfileUDID: normalizeString(secret.UserProfileUDID),
		CurBuvid:        normalizeString(secret.CurBuvid),
	}
}
