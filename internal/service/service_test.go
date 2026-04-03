package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"hi3loader/internal/bridge"
	"hi3loader/internal/config"
	"hi3loader/internal/loaderapiv1"
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

func TestSaveCredentialSettingsPreservesUnmodifiedSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Default()
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

	state, err := svc.SaveCredentialSettings("UpdatedName", "https://127.0.0.1:19777")
	if err != nil {
		t.Fatalf("save credential settings: %v", err)
	}

	if got := svc.Config().AsteriskName; got != "UpdatedName" {
		t.Fatalf("expected nickname to update, got %q", got)
	}
	if got := svc.Config().LoaderAPIBaseURL; got != "https://127.0.0.1:19777" {
		t.Fatalf("expected loader api url to update, got %q", got)
	}
	if got := state.Config.AsteriskName; got != "UpdatedName" {
		t.Fatalf("expected state nickname to update, got %q", got)
	}
	if got := state.Config.LoaderAPIBaseURL; got != "https://127.0.0.1:19777" {
		t.Fatalf("expected state loader api url to update, got %q", got)
	}
}

func TestSaveLauncherPathUpdatesConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	launcherPath := filepath.Join(dir, "launcher.exe")
	if err := os.WriteFile(launcherPath, []byte("stub"), 0o600); err != nil {
		t.Fatalf("write launcher.exe: %v", err)
	}
	cfg := config.Default()
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

	state, err := svc.SaveLauncherPath(launcherPath)
	if err != nil {
		t.Fatalf("save launcher path: %v", err)
	}
	if got := svc.Config().LauncherPath; got == "" {
		t.Fatal("expected launcher path to be saved")
	}
	if got := state.Config.LauncherPath; got == "" {
		t.Fatal("expected state launcher path to be saved")
	}
}

func TestClearCurrentAccountRemovesOnlyActiveEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Default()
	cfg.Accounts = []config.SavedAccount{
		{Account: "account-a", Password: "secret-a", AccessKey: "access-a", UName: "Alice", LastLoginSucc: true},
		{Account: "account-b", Password: "secret-b", AccessKey: "access-b", UName: "Bob", LastLoginSucc: true},
	}
	cfg.CurrentAccount = "account-b"
	cfg.AccountLogin = true
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

	state, err := svc.ClearCurrentAccount()
	if err != nil {
		t.Fatalf("clear current account: %v", err)
	}
	if len(state.Config.SavedAccounts) != 1 {
		t.Fatalf("expected one saved account to remain, got %d", len(state.Config.SavedAccounts))
	}
	if state.Config.Account != "account-a" {
		t.Fatalf("expected remaining account to become active, got %q", state.Config.Account)
	}
	if svc.Config().AccountLogin {
		t.Fatal("expected account login state to be cleared")
	}
}

func TestAdoptBundledLoaderAPIUpdatesEmptyConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Default()
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

	if err := svc.AdoptBundledLoaderAPI("https://127.0.0.1:50259"); err != nil {
		t.Fatalf("adopt bundled endpoint: %v", err)
	}
	if got := svc.Config().LoaderAPIBaseURL; got != "https://127.0.0.1:50259" {
		t.Fatalf("unexpected loader api base url: %q", got)
	}
}

func TestAdoptBundledLoaderAPIPreservesRemoteConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Default()
	cfg.LoaderAPIBaseURL = "https://example.com"
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

	if err := svc.AdoptBundledLoaderAPI("https://127.0.0.1:50259"); err != nil {
		t.Fatalf("adopt bundled endpoint: %v", err)
	}
	if got := svc.Config().LoaderAPIBaseURL; got != "https://example.com" {
		t.Fatalf("expected remote loader api url to remain unchanged, got %q", got)
	}
}

func TestBootstrapRequiresLoaderAPIAddress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Default()
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

	state, err := svc.Bootstrap(context.Background())
	if err == nil {
		t.Fatal("expected bootstrap without loader api address to fail")
	}
	if state.Running != true {
		t.Fatalf("expected service to start monitor before prompting config")
	}
	if state.LastErrorMessage.Code != "backend.error.loader_api_required" {
		t.Fatalf("unexpected error code: %+v", state.LastErrorMessage)
	}
}

func TestBootstrapReturnsWhileBundledRuntimeWarmsUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Default()
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	loaderCorePath := filepath.Join(dir, "loader-core.exe")
	sourcePath := filepath.Join(dir, "loader-core.go")
	source := `package main
import "time"
func main() { time.Sleep(2 * time.Second) }
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write loader-core stub: %v", err)
	}
	build := exec.Command("go", "build", "-o", loaderCorePath, sourcePath)
	build.Dir = dir
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build loader-core stub: %v (%s)", err, strings.TrimSpace(string(output)))
	}

	authorityPublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate authority key: %v", err)
	}
	restoreAuthority := bridge.SetEmbeddedServerAuthorityForTesting(authorityPublicKey)
	defer restoreAuthority()

	svc, err := New(path)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.module = &moduleRuntime{
		baseDir: dir,
		exePath: loaderCorePath,
	}
	defer func() {
		_ = svc.Close(context.Background())
	}()

	startedAt := time.Now()
	state, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed >= 2*time.Second {
		t.Fatalf("expected bootstrap to return before bundled runtime is ready, took %s", elapsed)
	}
	if !state.RuntimePreparing {
		t.Fatalf("expected runtimePreparing during bundled startup")
	}
	if state.APIReady {
		t.Fatalf("expected apiReady to remain false while bundled runtime is still starting")
	}
}

func TestPollAPIHealthOnceUpdatesReadyState(t *testing.T) {
	server, authorityPublicKey := newSecureHealthzServer(t)
	defer server.Close()
	restoreAuthority := bridge.SetEmbeddedServerAuthorityForTesting(authorityPublicKey)
	defer restoreAuthority()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Default()
	cfg.LoaderAPIBaseURL = server.URL
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

	svc.pollAPIHealthOnce(context.Background())
	if !svc.State().APIReady {
		t.Fatal("expected apiReady to become true after successful probe")
	}

	server.Close()
	svc.pollAPIHealthOnce(context.Background())
	if svc.State().APIReady {
		t.Fatal("expected apiReady to become false after probe failure")
	}
}

func TestPollAPIHealthOnceSkipsDuringAPIInteraction(t *testing.T) {
	serverHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHit = true
		http.NotFound(w, r)
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := config.Default()
	cfg.LoaderAPIBaseURL = server.URL
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

	svc.beginAPIInteraction()
	defer svc.endAPIInteraction()

	svc.pollAPIHealthOnce(context.Background())
	if serverHit {
		t.Fatal("expected poll to skip active api interaction")
	}
	if svc.State().APIReady {
		t.Fatal("expected apiReady to remain false when poll is skipped")
	}
}

func newSecureHealthzServer(t *testing.T) (*httptest.Server, ed25519.PublicKey) {
	t.Helper()

	curve := ecdh.X25519()
	authorityPublicKey, authorityPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate authority key: %v", err)
	}
	var (
		mu       sync.Mutex
		sessions = map[string][]byte{}
		server   *httptest.Server
	)

	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/session/manifest":
			req := &loaderapiv1.ManifestRequest{}
			if err := decodeServiceProtoRequest(r, req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fingerprint := sha256HexForTest(server.Certificate().Raw)
			generatedAt := time.Now().Format(time.RFC3339)
			signature := ed25519.Sign(authorityPrivateKey, manifestSignaturePayloadForTest("test", fingerprint, loaderapiv1.ProtocolValue, "module", generatedAt))
			writeServiceProtoResponse(t, w, &loaderapiv1.ManifestResponse{
				KeyId:           "test",
				TlsCertSha256:   fingerprint,
				ProtocolVersion: loaderapiv1.ProtocolValue,
				ServerName:      "module",
				GeneratedAt:     generatedAt,
				Signature:       signature,
			})
		case "/v1/session/handshake":
			req := &loaderapiv1.HandshakeRequest{}
			if err := decodeServiceProtoRequest(r, req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			clientPublicKey, err := curve.NewPublicKey(req.ClientPublicKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			serverKey, err := curve.GenerateKey(rand.Reader)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			serverNonce := make([]byte, 32)
			if _, err := io.ReadFull(rand.Reader, serverNonce); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sharedSecret, err := serverKey.ECDH(clientPublicKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sessionKey, err := hkdf.Key(sha256.New, sharedSecret, append(append([]byte(nil), req.ClientNonce...), serverNonce...), "hi3loader/loader-api/session/v1", 32)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sessionID := "sess-" + sha256HexForTest(serverNonce)[:12]
			mu.Lock()
			sessions[sessionID] = sessionKey
			mu.Unlock()
			writeServiceProtoResponse(t, w, &loaderapiv1.HandshakeResponse{
				SessionID:            sessionID,
				ServerPublicKey:      serverKey.PublicKey().Bytes(),
				ServerNonce:          serverNonce,
				ProtocolVersion:      loaderapiv1.ProtocolValue,
				ServerVersion:        "test",
				Message:              "OK",
				SessionExpiresUnixMs: time.Now().Add(30 * time.Second).UnixMilli(),
			})
		case "/v1/healthz":
			wire := &loaderapiv1.SealedRequest{}
			if err := decodeServiceProtoRequest(r, wire); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			sessionKey := append([]byte(nil), sessions[wire.SessionID]...)
			mu.Unlock()
			if len(sessionKey) == 0 {
				http.Error(w, "missing session", http.StatusUnauthorized)
				return
			}
			plaintext, err := openServicePayload(sessionKey, requestAADForTest("/v1/healthz", wire.SessionID), wire.Nonce, wire.Ciphertext)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			req := &loaderapiv1.HealthzRequest{}
			if err := proto.Unmarshal(plaintext, protoadapt.MessageV2Of(req)); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload, err := proto.Marshal(protoadapt.MessageV2Of(&loaderapiv1.HealthzResponse{
				Ok:              true,
				ServerName:      "module",
				ServerVersion:   "test",
				ProtocolVersion: loaderapiv1.ProtocolValue,
				Message:         "ok",
			}))
			if err != nil {
				t.Fatalf("marshal healthz response: %v", err)
			}
			nonce := make([]byte, 12)
			if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
				t.Fatalf("nonce: %v", err)
			}
			ciphertext, err := sealServicePayload(sessionKey, responseAADForTest("/v1/healthz", wire.SessionID), nonce, payload)
			if err != nil {
				t.Fatalf("seal response: %v", err)
			}
			writeServiceProtoResponse(t, w, &loaderapiv1.SealedResponse{
				Nonce:      nonce,
				Ciphertext: ciphertext,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	return server, authorityPublicKey
}

func decodeServiceProtoRequest(r *http.Request, out protoadapt.MessageV1) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return proto.Unmarshal(body, protoadapt.MessageV2Of(out))
}

func writeServiceProtoResponse(t *testing.T, w http.ResponseWriter, msg protoadapt.MessageV1) {
	t.Helper()
	raw, err := proto.Marshal(protoadapt.MessageV2Of(msg))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	w.Header().Set("Content-Type", loaderapiv1.ContentType)
	_, _ = w.Write(raw)
}

func requestAADForTest(path, sessionID string) []byte {
	return []byte("request|" + sessionID + "|" + path)
}

func responseAADForTest(path, sessionID string) []byte {
	return []byte("response|" + sessionID + "|" + path)
}

func sealServicePayload(key, aad, nonce, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nonce, plaintext, aad), nil
}

func openServicePayload(key, aad, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, aad)
}

func sha256HexForTest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func manifestSignaturePayloadForTest(keyID, tlsCertSHA256, protocolVersion, serverName, generatedAt string) []byte {
	parts := []string{
		strings.TrimSpace(keyID),
		strings.ToLower(strings.TrimSpace(tlsCertSHA256)),
		strings.TrimSpace(protocolVersion),
		strings.TrimSpace(serverName),
		strings.TrimSpace(generatedAt),
	}
	return []byte(strings.Join(parts, "\n"))
}
