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
