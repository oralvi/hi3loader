package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testNicknameUnicode = "\u4e0d\u60f3\u52a0\u5927\u73ed\u7684\u963f\u51db"
)

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

func TestSaveLoadEncryptsSensitiveFields(t *testing.T) {
	t.Setenv(storageSecretEnvVar, t.TempDir())
	t.Setenv(machineIDEnvVar, "test-machine-id")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := Default()
	cfg.Account = "tester"
	cfg.Password = "secret-password"
	cfg.AccessKey = "secret-access-key"
	cfg.CurrentAccount = "tester"
	cfg.BILIHITOKEN = "secret-bili-hitoken"
	cfg.SetDispatchSnapshot("8.7.0", DispatchCacheEntry{
		Data:          "secret-dispatch-data",
		Source:        "preferred_dispatch",
		RawLen:        123,
		DecodedLen:    456,
		DecodedSHA256: "abc123",
		SavedAt:       "2026-03-19T13:00:00Z",
	})
	cfg.Accounts = []SavedAccount{
		{
			Account:       "tester-alt",
			Password:      "secret-alt-password",
			UID:           424242,
			AccessKey:     "secret-alt-access-key",
			UName:         "AltUser",
			LastLoginSucc: true,
		},
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
		cfg.Password,
		cfg.AccessKey,
		cfg.BILIHITOKEN,
		cfg.DispatchData,
		cfg.Accounts[0].Password,
		cfg.Accounts[0].AccessKey,
	} {
		if strings.Contains(text, secret) {
			t.Fatalf("secret %q was written in plaintext", secret)
		}
	}
	if !strings.Contains(text, storageEnvelopePrefix) {
		t.Fatalf("expected encrypted payload marker in stored config")
	}
	parsed := readJSONMap(t, path)
	for _, key := range []string{"account", "password", "access_key", "uid", "uname", "last_login_succ", "account_login"} {
		if _, ok := parsed[key]; ok {
			t.Fatalf("expected root legacy auth field %q to be omitted from stored config", key)
		}
	}
	if !strings.Contains(text, `"current_account": "tester"`) {
		t.Fatalf("expected stored config to keep current_account")
	}

	loaded, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.Password != "secret-password" {
		t.Fatalf("unexpected password: %q", loaded.Password)
	}
	if loaded.AccessKey != "secret-access-key" {
		t.Fatalf("unexpected access key: %q", loaded.AccessKey)
	}
	if loaded.BILIHITOKEN != "secret-bili-hitoken" {
		t.Fatalf("unexpected bilihitoken: %q", loaded.BILIHITOKEN)
	}
	if loaded.DispatchData != "secret-dispatch-data" {
		t.Fatalf("unexpected dispatch data: %q", loaded.DispatchData)
	}
	if loaded.DispatchVersion != "8.7.0_gf_android_bilibili" {
		t.Fatalf("unexpected dispatch version: %q", loaded.DispatchVersion)
	}
	if loaded.DispatchSource != "preferred_dispatch" {
		t.Fatalf("unexpected dispatch source: %q", loaded.DispatchSource)
	}
	if loaded.CurrentAccount != "tester" {
		t.Fatalf("unexpected current account: %q", loaded.CurrentAccount)
	}
	if len(loaded.Accounts) != 2 {
		t.Fatalf("expected two saved accounts, got %d", len(loaded.Accounts))
	}
	var foundAlt bool
	for _, account := range loaded.Accounts {
		if account.Account != "tester-alt" {
			continue
		}
		foundAlt = true
		if account.Password != "secret-alt-password" {
			t.Fatalf("unexpected saved account password: %q", account.Password)
		}
		if account.AccessKey != "secret-alt-access-key" {
			t.Fatalf("unexpected saved account access key: %q", account.AccessKey)
		}
	}
	if !foundAlt {
		t.Fatal("expected migrated saved account entry for tester-alt")
	}
}

func TestLoadOrCreateBacksUpCorruptConfig(t *testing.T) {
	t.Setenv(storageSecretEnvVar, t.TempDir())
	t.Setenv(machineIDEnvVar, "test-machine-id")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("seed corrupt config: %v", err)
	}

	cfg, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load corrupt config: %v", err)
	}
	if cfg == nil {
		t.Fatalf("expected default config after corrupt load")
	}

	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil {
		t.Fatalf("glob corrupt backups: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one corrupt backup, got %d", len(matches))
	}
}

func TestLegacyPlaintextConfigMigratesToEncryptedStorage(t *testing.T) {
	t.Setenv(storageSecretEnvVar, t.TempDir())
	t.Setenv(machineIDEnvVar, "test-machine-id")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	legacy := `{
  "account": "tester",
  "password": "legacy-password",
  "access_key": "legacy-access-key",
  "BILIHITOKEN": "legacy-hitoken",
  "dispatch_data": "legacy-dispatch",
  "dispatch_cache": {
    "8.7.0_gf_android_bilibili": {
      "data": "legacy-dispatch",
      "source": "legacy_cache",
      "saved_at": "2026-03-19T10:00:00Z"
    }
  }
}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	cfg, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load legacy config: %v", err)
	}
	if cfg.Password != "legacy-password" || cfg.AccessKey != "legacy-access-key" || cfg.BILIHITOKEN != "legacy-hitoken" || cfg.DispatchData != "legacy-dispatch" {
		t.Fatalf("legacy config did not round-trip through migration")
	}
	if cfg.DispatchVersion != "8.7.0_gf_android_bilibili" {
		t.Fatalf("legacy dispatch cache was not migrated")
	}
	if cfg.DispatchSource != "legacy_cache" {
		t.Fatalf("legacy dispatch source was not preserved")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	text := string(raw)
	for _, secret := range []string{"legacy-password", "legacy-access-key", "legacy-hitoken", "legacy-dispatch"} {
		if strings.Contains(text, secret) {
			t.Fatalf("legacy secret %q still present in plaintext", secret)
		}
	}
	if !strings.Contains(text, storageEnvelopePrefix) {
		t.Fatalf("expected migrated config to contain encrypted payload marker")
	}
	parsed := readJSONMap(t, path)
	if _, ok := parsed["account"]; ok {
		t.Fatalf("legacy root account should not remain after migration")
	}
	if strings.Contains(text, "\"dispatch_cache\"") {
		t.Fatalf("legacy dispatch_cache should not be persisted after migration")
	}
}

func TestLoadLooseTypedConfigRewritesCanonicalFormat(t *testing.T) {
	t.Setenv(storageSecretEnvVar, t.TempDir())
	t.Setenv(machineIDEnvVar, "test-machine-id")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	loose := "\ufeff{\n" +
		`  "account": 123456,` + "\n" +
		`  "sleep_time": "5",` + "\n" +
		`  "clip_check": "true",` + "\n" +
		`  "auto_close": "1",` + "\n" +
		`  "uid": "42",` + "\n" +
		`  "last_login_succ": "true",` + "\n" +
		`  "auto_clip": "false",` + "\n" +
		`  "account_login": "true",` + "\n" +
		`  "background_opacity": "0.55",` + "\n" +
		`  "panel_blur": "false",` + "\n" +
		`  "bh_ver": 8.7,` + "\n" +
		`  "auto_expand_qrcode": true,` + "\n" +
		`  "auto_refresh_expired_qr": true,` + "\n" +
		`  "ver": 5` + "\n" +
		`}`
	if err := os.WriteFile(path, []byte(loose), 0o600); err != nil {
		t.Fatalf("write loose config: %v", err)
	}

	cfg, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load loose config: %v", err)
	}

	if cfg.Account != "123456" {
		t.Fatalf("unexpected account: %q", cfg.Account)
	}
	if cfg.SleepTime != 5 {
		t.Fatalf("unexpected sleep time: %d", cfg.SleepTime)
	}
	if !cfg.ClipCheck || !cfg.AutoClose || cfg.AutoClip {
		t.Fatalf("unexpected boolean coercion: clip=%v close=%v autoClip=%v", cfg.ClipCheck, cfg.AutoClose, cfg.AutoClip)
	}
	if cfg.UID != 42 {
		t.Fatalf("unexpected uid: %d", cfg.UID)
	}
	if cfg.BackgroundOpacity != 0.55 {
		t.Fatalf("unexpected background opacity: %v", cfg.BackgroundOpacity)
	}
	if cfg.PanelBlur {
		t.Fatalf("expected panel blur false after coercion")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rewritten config: %v", err)
	}
	text := string(raw)
	for _, key := range []string{"auto_expand_qrcode", "auto_refresh_expired_qr", "\"ver\""} {
		if strings.Contains(text, key) {
			t.Fatalf("deprecated key %q should not remain after rewrite", key)
		}
	}
	if !strings.Contains(text, "\"sleep_time\": 5") {
		t.Fatalf("expected canonical numeric sleep_time in rewritten config")
	}
	if !strings.Contains(text, "\"clip_check\": true") {
		t.Fatalf("expected canonical boolean clip_check in rewritten config")
	}
	if strings.Contains(text, "\"clip_check\": \"true\"") {
		t.Fatalf("stringified clip_check should have been normalized")
	}
	if strings.HasPrefix(text, "\ufeff") {
		t.Fatalf("utf-8 BOM should have been removed during rewrite")
	}
}

func TestLoadConfigDropsLegacyDispatchCacheAfterRewrite(t *testing.T) {
	t.Setenv(storageSecretEnvVar, t.TempDir())
	t.Setenv(machineIDEnvVar, "test-machine-id")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	legacy := `{
  "account": "tester",
  "dispatch_data": "legacy-dispatch",
  "dispatch_cache": {
    "8.7.0_gf_android_bilibili": {
      "data": "legacy-dispatch",
      "source": "legacy_cache",
      "saved_at": "2026-03-19T10:00:00Z"
    }
  },
  "auto_expand_qrcode": true
}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	cfg, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DispatchVersion != "8.7.0_gf_android_bilibili" {
		t.Fatalf("expected migrated dispatch version, got %q", cfg.DispatchVersion)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rewritten config: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, "\"dispatch_cache\"") {
		t.Fatalf("legacy dispatch_cache should not remain after rewrite")
	}
	if strings.Contains(text, "auto_expand_qrcode") {
		t.Fatalf("deprecated experimental key should not remain after rewrite")
	}
}

func TestDefaultNicknameIsPopulatedAndNormalized(t *testing.T) {
	cfg := Default()
	if cfg.AsteriskName != DefaultAsteriskName {
		t.Fatalf("unexpected default nickname: %q", cfg.AsteriskName)
	}

	cfg.AsteriskName = ""
	if !cfg.Normalize() {
		t.Fatalf("expected normalize to restore missing nickname")
	}
	if cfg.AsteriskName != DefaultAsteriskName {
		t.Fatalf("unexpected normalized nickname: %q", cfg.AsteriskName)
	}
}

func TestSaveLoadPreservesCustomNickname(t *testing.T) {
	t.Setenv(storageSecretEnvVar, t.TempDir())
	t.Setenv(machineIDEnvVar, "test-machine-id")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := Default()
	cfg.AsteriskName = testNicknameUnicode

	if err := Save(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.AsteriskName != testNicknameUnicode {
		t.Fatalf("unexpected loaded nickname: %q", loaded.AsteriskName)
	}
}

func TestNormalizeMigratesCurrentAccountIntoSavedAccounts(t *testing.T) {
	cfg := Default()
	cfg.Account = "tester"
	cfg.Password = "secret"
	cfg.UID = 1024
	cfg.AccessKey = "access"
	cfg.UName = "Tester"
	cfg.LastLoginSucc = true

	if !cfg.Normalize() {
		t.Fatalf("expected normalize to migrate current account into saved accounts")
	}
	if len(cfg.Accounts) != 1 {
		t.Fatalf("expected one saved account, got %d", len(cfg.Accounts))
	}
	if cfg.Accounts[0].Account != "tester" {
		t.Fatalf("unexpected saved account id: %q", cfg.Accounts[0].Account)
	}
}
