package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hi3loader/internal/config"
)

func TestExpirePendingCredentialsClearsCaptchaState(t *testing.T) {
	s := &Service{cfg: config.Default()}
	s.storePendingCredentials("demo", "secret")

	s.mu.Lock()
	gen := s.pendingPasswordGen
	s.captchaPending = true
	s.captchaURL = "http://127.0.0.1/captcha"
	s.lastAction = "captcha_required"
	s.mu.Unlock()

	s.expirePendingCredentials(gen)

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.pendingAccount != "" || len(s.pendingPassword) != 0 {
		t.Fatal("expected pending credentials to be cleared")
	}
	if s.captchaPending {
		t.Fatal("expected captcha pending flag to be cleared")
	}
	if s.captchaURL != "" {
		t.Fatal("expected captcha url to be cleared")
	}
	if s.lastAction != "captcha_expired" {
		t.Fatalf("expected lastAction to be captcha_expired, got %q", s.lastAction)
	}
}

func TestPendingCredentialsExpiredOnRead(t *testing.T) {
	s := &Service{cfg: config.Default()}
	s.storePendingCredentials("demo", "secret")

	s.mu.Lock()
	s.pendingPasswordTTL = time.Now().Add(-time.Second)
	s.mu.Unlock()

	account, password, ok := s.pendingCredentials()
	if ok || account != "" || len(password) != 0 {
		t.Fatal("expected expired pending credentials to be unavailable")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.pendingAccount != "" || len(s.pendingPassword) != 0 {
		t.Fatal("expected expired pending credentials to be wiped")
	}
}

func TestLoginRejectsMissingCredentials(t *testing.T) {
	s := &Service{cfg: config.Default()}

	_, err := s.Login(context.Background(), "", "", false)
	if err == nil {
		t.Fatal("expected login without credentials to fail")
	}
}

func TestUpdateBackgroundRejectsNonImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "not-image.txt")
	if err := os.WriteFile(path, []byte("plain text"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	s := &Service{cfg: config.Default()}
	if _, err := s.UpdateBackground(path, 0.4); err == nil {
		t.Fatal("expected non-image background update to fail")
	}
}

func TestSelectSavedAccountAppliesSavedSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Default()
	cfg.Accounts = []config.SavedAccount{
		{
			Account:       "account-a",
			Password:      "secret-a",
			UID:           1001,
			AccessKey:     "access-a",
			UName:         "Alice",
			LastLoginSucc: true,
		},
		{
			Account:       "account-b",
			Password:      "secret-b",
			UID:           1002,
			AccessKey:     "access-b",
			UName:         "Bob",
			LastLoginSucc: true,
		},
	}
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	svc, err := New(path)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer func() {
		_ = svc.Close(context.Background())
	}()

	state, err := svc.SelectSavedAccount("account-b")
	if err != nil {
		t.Fatalf("select saved account: %v", err)
	}
	if state.Config.Account != "account-b" {
		t.Fatalf("unexpected active account: %q", state.Config.Account)
	}
	if !state.Config.HasPassword || !state.Config.HasAccessKey {
		t.Fatal("expected selected account credentials to be applied")
	}
}

func TestSelectSavedAccountRestartsMonitorContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Default()
	cfg.Accounts = []config.SavedAccount{
		{
			Account:       "account-a",
			Password:      "secret-a",
			UID:           1001,
			AccessKey:     "access-a",
			UName:         "Alice",
			LastLoginSucc: true,
		},
		{
			Account:       "account-b",
			Password:      "secret-b",
			UID:           1002,
			AccessKey:     "access-b",
			UName:         "Bob",
			LastLoginSucc: true,
		},
	}
	cfg.ApplySavedAccount("account-a")
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	svc, err := New(path)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer func() {
		_ = svc.Close(context.Background())
	}()

	svc.mu.Lock()
	svc.recentTickets = map[string]time.Time{"stale": time.Now()}
	svc.clipboardHash = "hash"
	svc.windowMissStreak = 3
	svc.windowStaticStreak = 4
	svc.windowFingerprint = "fp"
	svc.lastNoticeCode = "backend.hint.qr_expand_manual"
	svc.lastNoticeAt = time.Now()
	svc.mu.Unlock()

	if _, err := svc.SelectSavedAccount("account-b"); err != nil {
		t.Fatalf("select saved account: %v", err)
	}

	svc.mu.RLock()
	defer svc.mu.RUnlock()
	if len(svc.recentTickets) != 0 {
		t.Fatalf("expected recent tickets to be cleared")
	}
	if svc.clipboardHash != "" {
		t.Fatalf("expected clipboard hash to be cleared")
	}
	if svc.windowMissStreak != 0 || svc.windowStaticStreak != 0 {
		t.Fatalf("expected monitor streaks to be reset")
	}
	if svc.windowFingerprint != "" {
		t.Fatalf("expected window fingerprint to be reset")
	}
	if svc.lastNoticeCode != "" || !svc.lastNoticeAt.IsZero() {
		t.Fatalf("expected last hint notice to be reset")
	}
}

func TestBridgeConfigSnapshotPreservesNickname(t *testing.T) {
	cfg := config.Default()
	cfg.AsteriskName = "\u4e0d\u60f3\u52a0\u5927\u73ed\u7684\u963f\u51db"

	snapshot := bridgeConfigSnapshot(cfg)
	if snapshot.AsteriskName != cfg.AsteriskName {
		t.Fatalf("expected snapshot nickname %q, got %q", cfg.AsteriskName, snapshot.AsteriskName)
	}

	roundTripped := snapshot.ToConfig()
	if roundTripped.AsteriskName != cfg.AsteriskName {
		t.Fatalf("expected round-tripped nickname %q, got %q", cfg.AsteriskName, roundTripped.AsteriskName)
	}
}

func TestSaveCredentialSettingsPreservesUnmodifiedSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Default()
	cfg.HI3UID = "123456789"
	cfg.BILIHITOKEN = "token-abc"
	cfg.AsteriskName = "OriginalName"
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	svc, err := New(path)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer func() {
		_ = svc.Close(context.Background())
	}()

	state, err := svc.SaveCredentialSettings("", false, "", false, "UpdatedName")
	if err != nil {
		t.Fatalf("save credential settings: %v", err)
	}

	if got := svc.Config().HI3UID; got != "123456789" {
		t.Fatalf("expected HI3UID to remain unchanged, got %q", got)
	}
	if got := svc.Config().BILIHITOKEN; got != "token-abc" {
		t.Fatalf("expected BILIHITOKEN to remain unchanged, got %q", got)
	}
	if got := svc.Config().AsteriskName; got != "UpdatedName" {
		t.Fatalf("expected nickname to update, got %q", got)
	}
	if got := state.Config.AsteriskName; got != "UpdatedName" {
		t.Fatalf("expected state nickname to update, got %q", got)
	}
}
