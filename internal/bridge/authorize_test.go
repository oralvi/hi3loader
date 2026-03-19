package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuthorizeHelperSecret(t *testing.T) {
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.txt")
	if err := os.WriteFile(authPath, []byte("secret-token"), 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	t.Setenv(helperTokenEnv, "secret-token")
	t.Setenv(helperAuthFileEnv, authPath)

	if err := authorizeHelperSecret(); err != nil {
		t.Fatalf("authorize helper secret: %v", err)
	}
	if _, err := os.Stat(authPath); !os.IsNotExist(err) {
		t.Fatalf("expected auth file to be removed, got err=%v", err)
	}
}

func TestAuthorizeHelperSecretRejectsMismatch(t *testing.T) {
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.txt")
	if err := os.WriteFile(authPath, []byte("secret-token"), 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}

	t.Setenv(helperTokenEnv, "other-token")
	t.Setenv(helperAuthFileEnv, authPath)

	if err := authorizeHelperSecret(); err == nil {
		t.Fatal("expected helper authorization to fail on mismatched token")
	}
	if _, err := os.Stat(authPath); !os.IsNotExist(err) {
		t.Fatalf("expected auth file to be removed after mismatch, got err=%v", err)
	}
}
