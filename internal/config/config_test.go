package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testNicknameUnicode = "不想加大班的阿凛"

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	parsed := map[string]any{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("decode config json: %v", err)
	}
	return parsed
}

func TestSaveLoadProtectsStoredSessionSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := Default()
	cfg.CurrentAccount = "tester"
	cfg.Accounts = []SavedAccount{
		{
			Account:       "tester",
			Password:      "secret-password",
			UID:           424242,
			AccessKey:     "secret-access-key",
			UName:         "Tester",
			LastLoginSucc: true,
		},
		{
			Account:       "tester-alt",
			Password:      "secret-alt-password",
			UID:           525252,
			AccessKey:     "secret-alt-access-key",
			UName:         "AltUser",
			LastLoginSucc: true,
		},
	}
	cfg.LoaderAPIBaseURL = "https://127.0.0.1:19777"
	cfg.DeviceProfile = DeviceProfile{
		AndroidID:       "84567e2dda72d1d4",
		MACAddress:      "08:00:27:53:DD:12",
		IMEI:            "227656364311444",
		RuntimeUDID:     "RUNTIMEUDID1234567890ABCDEFGH123456",
		UserProfileUDID: "XXA31CBAB6CBA63E432E087B58411A213BFB7",
		CurBuvid:        "XZA2FA4AC240F665E2F27F603ABF98C615C29",
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(raw)
	for _, secret := range []string{
		cfg.Accounts[0].Password,
		cfg.Accounts[0].AccessKey,
		cfg.Accounts[1].Password,
		cfg.Accounts[1].AccessKey,
		cfg.DeviceProfile.AndroidID,
		cfg.DeviceProfile.MACAddress,
		cfg.DeviceProfile.IMEI,
		cfg.DeviceProfile.RuntimeUDID,
		cfg.DeviceProfile.UserProfileUDID,
		cfg.DeviceProfile.CurBuvid,
	} {
		if strings.Contains(text, secret) {
			t.Fatalf("expected secret %q to stay out of stored config", secret)
		}
	}

	parsed := readJSONMap(t, path)
	for _, key := range []string{"account", "password", "access_key", "uid", "uname", "last_login_succ", "account_login"} {
		if _, ok := parsed[key]; ok {
			t.Fatalf("expected root auth field %q to be omitted from stored config", key)
		}
	}
	if strings.TrimSpace(StringValue(parsed["device_blob"])) == "" {
		t.Fatal("expected stored device blob")
	}
	accounts, ok := parsed["accounts"].([]any)
	if !ok || len(accounts) != 2 {
		t.Fatalf("expected 2 stored accounts, got %#v", parsed["accounts"])
	}
	for _, rawEntry := range accounts {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			t.Fatalf("expected stored account object, got %#v", rawEntry)
		}
		if _, hasPassword := entry["password"]; hasPassword {
			t.Fatalf("expected stored account password to be omitted: %#v", entry)
		}
		if _, hasAccessKey := entry["access_key"]; hasAccessKey {
			t.Fatalf("expected stored account access key to be omitted: %#v", entry)
		}
		if strings.TrimSpace(StringValue(entry["session_blob"])) == "" {
			t.Fatalf("expected stored account session blob: %#v", entry)
		}
	}

	loaded, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	active, ok := loaded.CurrentSavedAccount()
	if !ok {
		t.Fatal("expected active saved account")
	}
	if active.Account != "tester" {
		t.Fatalf("unexpected account: %q", active.Account)
	}
	if active.Password != "secret-password" {
		t.Fatalf("unexpected password: %q", active.Password)
	}
	if active.AccessKey != "secret-access-key" {
		t.Fatalf("unexpected access key: %q", active.AccessKey)
	}
	if loaded.LoaderAPIBaseURL != "https://127.0.0.1:19777" {
		t.Fatalf("unexpected loader api base url: %q", loaded.LoaderAPIBaseURL)
	}
	if loaded.DeviceProfile.AndroidID != cfg.DeviceProfile.AndroidID {
		t.Fatalf("unexpected device android id: %q", loaded.DeviceProfile.AndroidID)
	}
	if loaded.DeviceProfile.MACAddress != cfg.DeviceProfile.MACAddress {
		t.Fatalf("unexpected device mac: %q", loaded.DeviceProfile.MACAddress)
	}
	if len(loaded.Accounts) != 2 {
		t.Fatalf("expected 2 saved accounts, got %d", len(loaded.Accounts))
	}
}

func TestLoadOrCreateBacksUpCorruptConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("seed corrupt config: %v", err)
	}

	cfg, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected default config after corrupt load")
	}

	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil {
		t.Fatalf("glob corrupt backups: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one corrupt backup, got %d", len(matches))
	}
}

func TestLoadOrCreateResetsUnsupportedLegacyKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	legacy := `{
  "current_account": "tester",
  "dispatch_data": "legacy-dispatch"
}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	cfg, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.CurrentAccount != "" {
		t.Fatalf("expected reset config after unsupported keys, got current account %q", cfg.CurrentAccount)
	}

	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil {
		t.Fatalf("glob corrupt backups: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected unsupported config to be backed up, got %d backups", len(matches))
	}

	parsed := readJSONMap(t, path)
	if _, ok := parsed["dispatch_data"]; ok {
		t.Fatal("expected unsupported key to be removed after reset")
	}
}

func TestDefaultNicknameAndLoaderAPIBaseURL(t *testing.T) {
	cfg := Default()
	if cfg.AsteriskName != DefaultAsteriskName {
		t.Fatalf("unexpected default nickname: %q", cfg.AsteriskName)
	}
	if cfg.LoaderAPIBaseURL != "" {
		t.Fatalf("expected loader api url to default empty, got %q", cfg.LoaderAPIBaseURL)
	}

	cfg.AsteriskName = ""
	if !cfg.Normalize() {
		t.Fatal("expected normalize to restore missing nickname")
	}
	if cfg.AsteriskName != DefaultAsteriskName {
		t.Fatalf("unexpected normalized nickname: %q", cfg.AsteriskName)
	}
}

func TestNormalizeSelectsFirstSavedAccountAsCurrent(t *testing.T) {
	cfg := Default()
	cfg.Accounts = []SavedAccount{
		{
			Account:       "tester",
			Password:      "secret",
			UID:           1024,
			AccessKey:     "access",
			UName:         "Tester",
			LastLoginSucc: true,
		},
	}
	cfg.AsteriskName = testNicknameUnicode

	if !cfg.Normalize() {
		t.Fatal("expected normalize to select the first saved account")
	}
	if cfg.CurrentAccount != "tester" {
		t.Fatalf("unexpected current account: %q", cfg.CurrentAccount)
	}
	if cfg.AsteriskName != testNicknameUnicode {
		t.Fatalf("unexpected nickname after normalize: %q", cfg.AsteriskName)
	}
}

func TestGenerateDeviceProfileProducesCompleteProfile(t *testing.T) {
	profile, err := GenerateDeviceProfile()
	if err != nil {
		t.Fatalf("GenerateDeviceProfile() error = %v", err)
	}
	if !profile.IsComplete() {
		t.Fatalf("expected generated profile to be complete: %#v", profile)
	}
	if len(profile.AndroidID) != 16 {
		t.Fatalf("unexpected android id length: %q", profile.AndroidID)
	}
	if len(profile.MACAddress) != 17 {
		t.Fatalf("unexpected mac length: %q", profile.MACAddress)
	}
	if len(profile.IMEI) != 15 {
		t.Fatalf("unexpected imei length: %q", profile.IMEI)
	}
	if len(profile.RuntimeUDID) != 36 {
		t.Fatalf("unexpected runtime udid length: %q", profile.RuntimeUDID)
	}
	if len(profile.UserProfileUDID) != 37 {
		t.Fatalf("unexpected user profile udid length: %q", profile.UserProfileUDID)
	}
	if len(profile.CurBuvid) != 37 {
		t.Fatalf("unexpected buvid length: %q", profile.CurBuvid)
	}
}
