package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadEncryptsSensitiveFields(t *testing.T) {
	t.Setenv(storageSecretEnvVar, t.TempDir())
	t.Setenv(machineIDEnvVar, "test-machine-id")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := Default()
	cfg.Account = "tester"
	cfg.Password = "secret-password"
	cfg.AccessKey = "secret-access-key"
	cfg.BILIHITOKEN = "secret-bili-hitoken"
	cfg.SetDispatchSnapshot("8.7.0", DispatchCacheEntry{
		Data:          "secret-dispatch-data",
		Source:        "preferred_dispatch",
		RawLen:        123,
		DecodedLen:    456,
		DecodedSHA256: "abc123",
		SavedAt:       "2026-03-19T13:00:00Z",
	})

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
	} {
		if strings.Contains(text, secret) {
			t.Fatalf("secret %q was written in plaintext", secret)
		}
	}
	if !strings.Contains(text, storageEnvelopePrefix) {
		t.Fatalf("expected encrypted payload marker in stored config")
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
	if strings.Contains(text, "\"dispatch_cache\"") {
		t.Fatalf("legacy dispatch_cache should not be persisted after migration")
	}
}
