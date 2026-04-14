package config

import (
	"encoding/base64"
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
	"launcher_path":       {},
	"asterisk_name":       {},
	"accounts":            {},
	"auto_window_capture": {},
	"loader_api_base_url": {},
	"background_image":    {},
	"background_opacity":  {},
	"panel_blur":          {},
	"device_blob":         {},
}

type storedConfigJSON struct {
	CurrentAccount    string                   `json:"current_account,omitempty"`
	SleepTime         int                      `json:"sleep_time"`
	AutoClose         bool                     `json:"auto_close"`
	GamePath          string                   `json:"game_path,omitempty"`
	LauncherPath      string                   `json:"launcher_path,omitempty"`
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
	Account           string `json:"account"`
	SessionBlob       string `json:"session_blob,omitempty"`
	RememberPassword  bool   `json:"remember_password,omitempty"`
	EncryptedPassword []byte `json:"encrypted_password,omitempty"`
	UID               int64  `json:"uid,omitempty"`
	UName             string `json:"uname,omitempty"`
	LastLoginSucc     bool   `json:"last_login_succ,omitempty"`
}

type sessionSecretEnvelope struct {
	AccessKey string                `json:"access_key,omitempty"`
	Device    *deviceSecretEnvelope `json:"device,omitempty"`
}

type deviceSecretEnvelope struct {
	PresetID        string `json:"preset_id,omitempty"`
	Model           string `json:"model,omitempty"`
	Brand           string `json:"brand,omitempty"`
	SupportABIs     string `json:"support_abis,omitempty"`
	Display         string `json:"display,omitempty"`
	AndroidID       string `json:"android_id,omitempty"`
	MACAddress      string `json:"mac,omitempty"`
	IMEI            string `json:"imei,omitempty"`
	RuntimeUDID     string `json:"runtime_udid,omitempty"`
	UserProfileUDID string `json:"user_profile_udid,omitempty"`
	CurBuvid        string `json:"cur_buvid,omitempty"`
}

func (c *Codec) encodeStoredConfig(cfg *Config) (*storedConfigJSON, error) {
	clone := cfg.Clone()
	if clone == nil {
		return nil, fmt.Errorf("encode config: config is nil")
	}
	accounts, err := c.encodeStoredSavedAccounts(clone.Accounts)
	if err != nil {
		return nil, err
	}
	deviceBlob, err := c.encodeStoredDeviceProfile(clone.DeviceProfile)
	if err != nil {
		return nil, err
	}
	return &storedConfigJSON{
		CurrentAccount:    normalizeString(clone.CurrentAccount),
		SleepTime:         clone.SleepTime,
		AutoClose:         clone.AutoClose,
		GamePath:          normalizeString(clone.GamePath),
		LauncherPath:      normalizeString(clone.LauncherPath),
		AsteriskName:      normalizeString(clone.AsteriskName),
		Accounts:          accounts,
		AutoWindowCapture: clone.AutoWindowCapture,
		LoaderAPIBaseURL:  normalizeString(clone.LoaderAPIBaseURL),
		BackgroundImage:   normalizeString(clone.BackgroundImage),
		BackgroundOpacity: clone.BackgroundOpacity,
		PanelBlur:         clone.PanelBlur,
		DeviceBlob:        deviceBlob,
	}, nil
}

func (c *Codec) decodeStoredConfig(raw []byte) (*Config, bool, error) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil, false, fmt.Errorf("decode config: %w", err)
	}
	for key := range rawMap {
		if _, ok := allowedConfigKeys[key]; !ok {
			return nil, false, fmt.Errorf("decode config: unsupported key %q", key)
		}
	}

	stored := storedConfigJSON{
		SleepTime:         Default().SleepTime,
		AsteriskName:      Default().AsteriskName,
		BackgroundOpacity: Default().BackgroundOpacity,
		PanelBlur:         Default().PanelBlur,
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, false, fmt.Errorf("decode config: %w", err)
	}
	accounts, migrated, err := c.decodeStoredSavedAccounts(stored.Accounts)
	if err != nil {
		return nil, false, err
	}
	return &Config{
		CurrentAccount:    stored.CurrentAccount,
		SleepTime:         stored.SleepTime,
		AutoClose:         stored.AutoClose,
		GamePath:          stored.GamePath,
		LauncherPath:      stored.LauncherPath,
		AsteriskName:      stored.AsteriskName,
		Accounts:          accounts,
		AutoWindowCapture: stored.AutoWindowCapture,
		LoaderAPIBaseURL:  stored.LoaderAPIBaseURL,
		BackgroundImage:   stored.BackgroundImage,
		BackgroundOpacity: stored.BackgroundOpacity,
		PanelBlur:         stored.PanelBlur,
		DeviceProfile:     c.decodeStoredDeviceProfile(stored.DeviceBlob),
	}, migrated, nil
}

func backupCorruptConfig(path string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	backupPath := fmt.Sprintf("%s.corrupt-%s", path, time.Now().Format("20060102-150405"))
	return AtomicWriteFile(backupPath, data, 0o600)
}

func (c *Codec) encodeStoredSavedAccounts(accounts []SavedAccount) ([]storedSavedAccountJSON, error) {
	if len(accounts) == 0 {
		return nil, nil
	}
	stored := make([]storedSavedAccountJSON, 0, len(accounts))
	for _, entry := range accounts {
		encoded, err := c.encodeStoredSavedAccount(entry)
		if err != nil {
			return nil, err
		}
		stored = append(stored, encoded)
	}
	return stored, nil
}

func (c *Codec) encodeStoredSavedAccount(entry SavedAccount) (storedSavedAccountJSON, error) {
	stored := storedSavedAccountJSON{
		Account:          normalizeString(entry.Account),
		RememberPassword: entry.RememberPassword,
		UID:              entry.UID,
		UName:            normalizeString(entry.UName),
		LastLoginSucc:    entry.LastLoginSucc,
	}
	if entry.RememberPassword && strings.TrimSpace(entry.Password) != "" {
		passwordRaw := []byte(normalizeString(entry.Password))
		passwordBlob, err := c.secretStore.Protect(passwordRaw)
		for i := range passwordRaw {
			passwordRaw[i] = 0
		}
		if err != nil {
			return stored, fmt.Errorf("protect password blob: %w", err)
		}
		stored.EncryptedPassword = passwordBlob
	}
	if strings.TrimSpace(entry.AccessKey) == "" && !entry.DeviceProfile.IsComplete() {
		return stored, nil
	}
	secret := sessionSecretEnvelope{
		AccessKey: normalizeString(entry.AccessKey),
	}
	if envelope := newDeviceSecretEnvelope(entry.DeviceProfile); envelope != nil {
		secret.Device = envelope
	}
	raw, err := json.Marshal(secret)
	if err != nil {
		return stored, fmt.Errorf("encode session blob: %w", err)
	}
	blob, err := c.protectBlob(raw)
	if err != nil {
		return stored, fmt.Errorf("protect session blob: %w", err)
	}
	stored.SessionBlob = blob
	return stored, nil
}

func (c *Codec) decodeStoredSavedAccounts(accounts []storedSavedAccountJSON) ([]SavedAccount, bool, error) {
	if len(accounts) == 0 {
		return nil, false, nil
	}
	decoded := make([]SavedAccount, 0, len(accounts))
	migrated := false
	for _, entry := range accounts {
		account, accountMigrated, err := c.decodeStoredSavedAccount(entry)
		if err != nil {
			return nil, false, err
		}
		decoded = append(decoded, account)
		migrated = migrated || accountMigrated
	}
	return decoded, migrated, nil
}

func (c *Codec) decodeStoredSavedAccount(entry storedSavedAccountJSON) (SavedAccount, bool, error) {
	decoded := SavedAccount{
		Account:          normalizeString(entry.Account),
		RememberPassword: entry.RememberPassword,
		UID:              entry.UID,
		UName:            normalizeString(entry.UName),
		LastLoginSucc:    entry.LastLoginSucc,
	}
	migrated := false
	if entry.RememberPassword && len(entry.EncryptedPassword) > 0 {
		password, err := c.secretStore.Unprotect(entry.EncryptedPassword)
		if err == nil {
			decoded.Password = normalizeString(string(password))
			for i := range password {
				password[i] = 0
			}
		}
	}
	if strings.TrimSpace(entry.SessionBlob) == "" {
		return decoded, migrated, nil
	}
	raw, err := c.unprotectBlob(entry.SessionBlob)
	if err != nil {
		decoded.LastLoginSucc = false
		return decoded, migrated, nil
	}
	defer wipeBytes(raw)
	var secret sessionSecretEnvelope
	if err := json.Unmarshal(raw, &secret); err != nil {
		decoded.LastLoginSucc = false
		return decoded, migrated, nil
	}
	var legacy struct {
		Password string `json:"password,omitempty"`
	}
	if err := json.Unmarshal(raw, &legacy); err == nil && strings.TrimSpace(decoded.Password) == "" {
		if password := normalizeString(legacy.Password); password != "" {
			decoded.Password = password
			decoded.RememberPassword = true
			migrated = true
		}
	}
	decoded.AccessKey = normalizeString(secret.AccessKey)
	if secret.Device != nil {
		decoded.DeviceProfile = deviceProfileFromSecretEnvelope(*secret.Device)
	}
	if decoded.AccessKey == "" {
		decoded.LastLoginSucc = false
	}
	return decoded, migrated, nil
}

func (c *Codec) encodeStoredDeviceProfile(profile DeviceProfile) (string, error) {
	if !profile.IsComplete() {
		return "", nil
	}
	raw, err := json.Marshal(newDeviceSecretEnvelope(profile))
	if err != nil {
		return "", fmt.Errorf("encode device blob: %w", err)
	}
	blob, err := c.protectBlob(raw)
	if err != nil {
		return "", fmt.Errorf("protect device blob: %w", err)
	}
	return blob, nil
}

func (c *Codec) decodeStoredDeviceProfile(blob string) DeviceProfile {
	if strings.TrimSpace(blob) == "" {
		return DeviceProfile{}
	}
	raw, err := c.unprotectBlob(blob)
	if err != nil {
		return DeviceProfile{}
	}
	defer wipeBytes(raw)
	var secret deviceSecretEnvelope
	if err := json.Unmarshal(raw, &secret); err != nil {
		return DeviceProfile{}
	}
	return deviceProfileFromSecretEnvelope(secret)
}

func newDeviceSecretEnvelope(profile DeviceProfile) *deviceSecretEnvelope {
	if !profile.IsComplete() {
		return nil
	}
	return &deviceSecretEnvelope{
		PresetID:        normalizeString(profile.PresetID),
		Model:           normalizeString(profile.Model),
		Brand:           normalizeString(profile.Brand),
		SupportABIs:     normalizeString(profile.SupportABIs),
		Display:         normalizeString(profile.Display),
		AndroidID:       normalizeString(profile.AndroidID),
		MACAddress:      normalizeString(profile.MACAddress),
		IMEI:            normalizeString(profile.IMEI),
		RuntimeUDID:     normalizeString(profile.RuntimeUDID),
		UserProfileUDID: normalizeString(profile.UserProfileUDID),
		CurBuvid:        normalizeString(profile.CurBuvid),
	}
}

func deviceProfileFromSecretEnvelope(secret deviceSecretEnvelope) DeviceProfile {
	return DeviceProfile{
		PresetID:        normalizeString(secret.PresetID),
		Model:           normalizeString(secret.Model),
		Brand:           normalizeString(secret.Brand),
		SupportABIs:     normalizeString(secret.SupportABIs),
		Display:         normalizeString(secret.Display),
		AndroidID:       normalizeString(secret.AndroidID),
		MACAddress:      normalizeString(secret.MACAddress),
		IMEI:            normalizeString(secret.IMEI),
		RuntimeUDID:     normalizeString(secret.RuntimeUDID),
		UserProfileUDID: normalizeString(secret.UserProfileUDID),
		CurBuvid:        normalizeString(secret.CurBuvid),
	}
}

func (c *Codec) protectBlob(plaintext []byte) (string, error) {
	if c == nil || c.secretStore == nil {
		return "", fmt.Errorf("secret store is not configured")
	}
	blob, err := c.secretStore.Protect(plaintext)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(blob), nil
}

func (c *Codec) unprotectBlob(ciphertext string) ([]byte, error) {
	if c == nil || c.secretStore == nil {
		return nil, fmt.Errorf("secret store is not configured")
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode session blob: %w", err)
	}
	return c.secretStore.Unprotect(raw)
}
