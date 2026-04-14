package bridge

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"hi3loader/internal/buildinfo"
	"hi3loader/internal/loaderapiv1"
)

type loaderAPIClient struct {
	baseURL         string
	identityManager *IdentityManager
}

type loaderAPIDeployMode string

const (
	loaderAPIDeployLocal  loaderAPIDeployMode = "local"
	loaderAPIDeployRemote loaderAPIDeployMode = "remote"
)

type loaderAPISession struct {
	ID        string
	Key       []byte
	ExpiresAt time.Time
}

type loaderAPIManifestCacheEntry struct {
	Manifest *loaderapiv1.ManifestResponse
	CachedAt time.Time
}

type loaderAPISessionCacheEntry struct {
	Session     *loaderAPISession
	Fingerprint string
}

const (
	loaderAPIScanTimeout        = 20 * time.Second
	loaderAPIScanHeaderTimeout  = 20 * time.Second
	loaderAPIProbeTimeout       = 5 * time.Second
	loaderAPIProbeHeaderTimeout = 5 * time.Second
	loaderAPIProtocolInfo       = "hi3loader/loader-api/session/v1"
	loaderAPINonceSize          = 12
	loaderAPIHandshakeNonceSize = 32
	loaderAPISessionKeySize     = 32
	loaderAPIManifestCacheTTL   = 30 * time.Second
	loaderAPISessionDefaultTTL  = 90 * time.Second
	loaderAPIHTTPMaxIdleConns   = 10
	loaderAPIHTTPIdleTimeout    = 90 * time.Second
)

var loaderAPICache = struct {
	mu        sync.Mutex
	manifests map[string]loaderAPIManifestCacheEntry
	sessions  map[string]loaderAPISessionCacheEntry
}{
	manifests: map[string]loaderAPIManifestCacheEntry{},
	sessions:  map[string]loaderAPISessionCacheEntry{},
}

var loaderAPIHTTPClients sync.Map

func newLoaderAPIClient(baseURL string) *loaderAPIClient {
	return newLoaderAPIClientWithIdentityManager(baseURL, nil)
}

func newLoaderAPIClientWithIdentityManager(baseURL string, identityManager *IdentityManager) *loaderAPIClient {
	return &loaderAPIClient{
		baseURL:         strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		identityManager: identityManager,
	}
}

func newLoaderAPIHTTPClient(baseURL string, deployMode loaderAPIDeployMode, allowTCPFallback bool, pinnedFingerprint string, tlsConfig *tls.Config) *http.Client {
	cacheKey := strings.TrimSpace(baseURL) + "|" + string(deployMode) + "|" + fmt.Sprintf("%t", allowTCPFallback) + "|" + strings.TrimSpace(pinnedFingerprint)
	if cached, ok := loaderAPIHTTPClients.Load(cacheKey); ok {
		return cached.(*http.Client)
	}

	clonedTLSConfig := tlsConfig
	if tlsConfig != nil {
		clonedTLSConfig = tlsConfig.Clone()
	}
	transport := &http.Transport{
		Proxy:               nil,
		DialContext:         newLoaderAPIDialContext(baseURL, deployMode, allowTCPFallback),
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     clonedTLSConfig,
		MaxIdleConns:        loaderAPIHTTPMaxIdleConns,
		MaxIdleConnsPerHost: loaderAPIHTTPMaxIdleConns,
		IdleConnTimeout:     loaderAPIHTTPIdleTimeout,
	}

	client := &http.Client{
		Transport: transport,
	}
	actual, _ := loaderAPIHTTPClients.LoadOrStore(cacheKey, client)
	return actual.(*http.Client)
}

func loaderAPIDeployModeForRequest(baseURL string, meta ClientMeta) loaderAPIDeployMode {
	switch strings.ToLower(strings.TrimSpace(meta.TransportMode)) {
	case string(loaderAPIDeployLocal):
		return loaderAPIDeployLocal
	case string(loaderAPIDeployRemote):
		return loaderAPIDeployRemote
	default:
		if strings.EqualFold(strings.TrimSpace(transportModeForBaseURL(baseURL)), string(loaderAPIDeployRemote)) {
			return loaderAPIDeployRemote
		}
		return loaderAPIDeployLocal
	}
}

func withLoaderAPIRequestTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= timeout {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

func ProbeLoaderAPI(ctx context.Context, baseURL string, meta ClientMeta) error {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return fmt.Errorf("missing loader_api_url")
	}
	client := newLoaderAPIClient(baseURL)
	_, err := client.Healthz(ctx, meta)
	return err
}

func FetchRuntimeProfile(ctx context.Context, baseURL string, meta ClientMeta) (RuntimeProfile, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return RuntimeProfile{}, fmt.Errorf("missing loader_api_url")
	}
	client := newLoaderAPIClient(baseURL)
	resp, err := client.RuntimeProfile(ctx, meta)
	if err != nil {
		return RuntimeProfile{}, err
	}
	return RuntimeProfile{
		ChannelID:      resp.ChannelId,
		AppID:          resp.AppId,
		CPID:           strings.TrimSpace(resp.CpId),
		CPAppID:        strings.TrimSpace(resp.CpAppId),
		CPAppKey:       strings.TrimSpace(resp.CpAppKey),
		ServerID:       resp.ServerId,
		ChannelVersion: strings.TrimSpace(resp.ChannelVersion),
		GameVer:        strings.TrimSpace(resp.GameVer),
		VersionCode:    resp.VersionCode,
		SDKVer:         strings.TrimSpace(resp.SdkVer),
	}, nil
}

func (c *loaderAPIClient) Healthz(ctx context.Context, meta ClientMeta) (*loaderapiv1.HealthzResponse, error) {
	if err := c.validateBaseURL(); err != nil {
		return nil, err
	}
	req := &loaderapiv1.HealthzRequest{ClientMeta: toProtoClientMeta(meta)}
	resp := &loaderapiv1.HealthzResponse{}
	if err := c.performSealed(ctx, meta, loaderAPIProbeTimeout, loaderAPIProbeHeaderTimeout, "/v1/healthz", req, resp); err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("loader api healthz rejected request: %s", strings.TrimSpace(resp.Message))
	}
	return resp, nil
}

func (c *loaderAPIClient) RuntimeProfile(ctx context.Context, meta ClientMeta) (*loaderapiv1.RuntimeProfileResponse, error) {
	if err := c.validateBaseURL(); err != nil {
		return nil, err
	}
	req := &loaderapiv1.RuntimeProfileRequest{ClientMeta: toProtoClientMeta(meta)}
	resp := &loaderapiv1.RuntimeProfileResponse{}
	if err := c.performSealed(ctx, meta, loaderAPIProbeTimeout, loaderAPIProbeHeaderTimeout, "/v1/runtime/profile", req, resp); err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, fmt.Errorf("loader api runtime profile rejected request: %s", strings.TrimSpace(resp.Message))
	}
	return resp, nil
}

func (c *loaderAPIClient) ExecuteScan(ctx context.Context, req ScanRequest) (*loaderapiv1.ScanExecuteResponse, error) {
	if err := c.validateBaseURL(); err != nil {
		return nil, err
	}
	wireReq := &loaderapiv1.ScanExecuteRequest{
		ClientMeta:   toProtoClientMeta(req.ClientMeta),
		Ticket:       strings.TrimSpace(req.Ticket),
		UID:          req.UID,
		AccessKey:    strings.TrimSpace(req.AccessKey),
		AsteriskName: strings.TrimSpace(req.AsteriskName),
	}
	resp := &loaderapiv1.ScanExecuteResponse{}
	if err := c.performSealed(ctx, req.ClientMeta, loaderAPIScanTimeout, loaderAPIScanHeaderTimeout, "/v1/scan/execute", wireReq, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *loaderAPIClient) validateBaseURL() error {
	if strings.TrimSpace(c.baseURL) == "" {
		return fmt.Errorf("missing loader_api_url")
	}
	parsed, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse loader api url: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(parsed.Scheme), "https") {
		return fmt.Errorf("loader api url must use https")
	}
	if !HasEmbeddedServerAuthority() {
		return errMissingEmbeddedAuthority
	}
	return nil
}

func (c *loaderAPIClient) performSealed(ctx context.Context, meta ClientMeta, timeout, _ time.Duration, path string, reqMsg, respMsg protoadapt.MessageV1) error {
	attemptCtx, cancel := withLoaderAPIRequestTimeout(ctx, timeout)
	manifest, err := c.resolveManifest(attemptCtx, meta, false)
	if err != nil {
		cancel()
		return err
	}
	deployMode := loaderAPIDeployModeForRequest(c.baseURL, meta)
	httpClient := newLoaderAPIHTTPClient(c.baseURL, deployMode, false, normalizeServerCertFingerprint(manifest.TlsCertSha256), pinnedTLSConfig(manifest))
	session, err := c.handshake(attemptCtx, httpClient, meta, manifest, false)
	if err == nil {
		err = c.postSealed(attemptCtx, httpClient, path, session, reqMsg, respMsg)
	}
	cancel()
	if err == nil {
		return nil
	}

	c.invalidateSessionCache()
	c.invalidateManifestCache()

	retryCtx, retryCancel := withLoaderAPIRequestTimeout(ctx, timeout)
	manifest, refreshErr := c.resolveManifest(retryCtx, meta, true)
	if refreshErr != nil {
		retryCancel()
		return fmt.Errorf("%w; refresh manifest: %v", err, refreshErr)
	}
	httpClient = newLoaderAPIHTTPClient(c.baseURL, deployMode, false, normalizeServerCertFingerprint(manifest.TlsCertSha256), pinnedTLSConfig(manifest))
	session, refreshErr = c.handshake(retryCtx, httpClient, meta, manifest, true)
	if refreshErr != nil {
		retryCancel()
		return fmt.Errorf("%w; refresh handshake: %v", err, refreshErr)
	}
	if refreshErr = c.postSealed(retryCtx, httpClient, path, session, reqMsg, respMsg); refreshErr != nil {
		retryCancel()
		return fmt.Errorf("%w; retry request: %v", err, refreshErr)
	}
	retryCancel()
	return nil
}

func (c *loaderAPIClient) resolveManifest(ctx context.Context, meta ClientMeta, forceRefresh bool) (*loaderapiv1.ManifestResponse, error) {
	if !forceRefresh {
		if cached, ok := c.cachedManifest(); ok {
			return cached, nil
		}
	}

	deployMode := loaderAPIDeployModeForRequest(c.baseURL, meta)
	httpClient := newLoaderAPIHTTPClient(c.baseURL, deployMode, false, "manifest-insecure", insecureManifestTLSConfig())
	req := &loaderapiv1.ManifestRequest{ClientMeta: toProtoClientMeta(meta)}
	resp := &loaderapiv1.ManifestResponse{}
	httpResp, err := c.postProto(ctx, httpClient, "/v1/session/manifest", req, resp)
	if err != nil {
		return nil, err
	}
	if err := verifySignedManifest(resp); err != nil {
		return nil, err
	}
	if httpResp == nil || httpResp.TLS == nil || len(httpResp.TLS.PeerCertificates) == 0 {
		return nil, errInvalidManifest("missing peer certificate")
	}
	if !fingerprintMatchesCertificate(resp.TlsCertSha256, httpResp.TLS.PeerCertificates[0].Raw) {
		return nil, errInvalidManifest("tls certificate does not match signed manifest")
	}
	c.storeManifest(resp)
	return resp, nil
}

func (c *loaderAPIClient) handshake(ctx context.Context, httpClient *http.Client, meta ClientMeta, manifest *loaderapiv1.ManifestResponse, forceRefresh bool) (*loaderAPISession, error) {
	fingerprint := ""
	if manifest != nil {
		fingerprint = normalizeServerCertFingerprint(manifest.TlsCertSha256)
	}
	if !forceRefresh {
		if cached, ok := c.cachedSession(fingerprint); ok {
			return cached, nil
		}
	}

	curve := ecdh.X25519()
	clientKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate handshake key: %w", err)
	}
	clientNonce, err := randomBytes(loaderAPIHandshakeNonceSize)
	if err != nil {
		return nil, fmt.Errorf("generate handshake nonce: %w", err)
	}
	identity, err := c.loadClientIdentity()
	if err != nil {
		return nil, err
	}

	req := &loaderapiv1.HandshakeRequest{
		ClientMeta:          toProtoClientMeta(meta),
		ClientPublicKey:     clientKey.PublicKey().Bytes(),
		ClientNonce:         clientNonce,
		UnixMs:              time.Now().UnixMilli(),
		ClientIdentityKey:   append([]byte(nil), identity.PublicKey...),
		ClientIdentityKeyId: identity.keyID(),
	}
	req.ClientIdentitySig = identity.sign(handshakeSignaturePayload(req))

	resp := &loaderapiv1.HandshakeResponse{}
	if _, err := c.postProto(ctx, httpClient, "/v1/session/handshake", req, resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.SessionID) == "" {
		return nil, fmt.Errorf("loader api handshake missing session id")
	}
	if strings.TrimSpace(resp.ProtocolVersion) != "" && strings.TrimSpace(resp.ProtocolVersion) != loaderapiv1.ProtocolValue {
		return nil, fmt.Errorf("loader api protocol mismatch: %s", strings.TrimSpace(resp.ProtocolVersion))
	}
	serverPublicKey, err := curve.NewPublicKey(resp.ServerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode handshake server public key: %w", err)
	}
	sharedSecret, err := clientKey.ECDH(serverPublicKey)
	if err != nil {
		return nil, fmt.Errorf("derive handshake secret: %w", err)
	}
	defer wipeBytes(sharedSecret)
	sessionKey, err := hkdf.Key(sha256.New, sharedSecret, append(append([]byte(nil), clientNonce...), resp.ServerNonce...), loaderAPIProtocolInfo, loaderAPISessionKeySize)
	if err != nil {
		return nil, fmt.Errorf("derive session key: %w", err)
	}
	defer wipeBytes(sessionKey)
	expiresAt := time.Now().Add(loaderAPISessionDefaultTTL)
	if resp.SessionExpiresUnixMs > 0 {
		candidate := time.UnixMilli(resp.SessionExpiresUnixMs)
		if candidate.After(time.Now()) {
			expiresAt = candidate
		}
	}
	session := &loaderAPISession{
		ID:        strings.TrimSpace(resp.SessionID),
		Key:       append([]byte(nil), sessionKey...),
		ExpiresAt: expiresAt,
	}
	c.storeSession(fingerprint, session)
	return session, nil
}

func (c *loaderAPIClient) postProto(ctx context.Context, httpClient *http.Client, path string, reqMsg, respMsg protoadapt.MessageV1) (*http.Response, error) {
	raw, err := proto.Marshal(protoadapt.MessageV2Of(reqMsg))
	if err != nil {
		return nil, fmt.Errorf("marshal protobuf request: %w", err)
	}
	defer wipeBytes(raw)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("build loader api request: %w", err)
	}
	req.Header.Set("Content-Type", loaderapiv1.ContentType)
	req.Header.Set("Accept", loaderapiv1.ContentType)
	req.Header.Set("X-Loader-Proto", loaderapiv1.ProtocolValue)
	req.Header.Set("User-Agent", "hi3loader-helper/"+strings.TrimSpace(buildinfo.AppVersion))

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request loader api %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read loader api %s response: %w", path, err)
	}
	defer wipeBytes(body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(body) > 0 {
			return resp, fmt.Errorf("loader api %s returned %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
		}
		return resp, fmt.Errorf("loader api %s returned %s", path, resp.Status)
	}
	if err := proto.Unmarshal(body, protoadapt.MessageV2Of(respMsg)); err != nil {
		return resp, fmt.Errorf("decode protobuf response from %s: %w", path, err)
	}
	return resp, nil
}

func (c *loaderAPIClient) loadClientIdentity() (*clientIdentity, error) {
	if c != nil && c.identityManager != nil {
		return c.identityManager.Load()
	}
	return loadClientIdentity()
}

func (c *loaderAPIClient) postSealed(ctx context.Context, httpClient *http.Client, path string, session *loaderAPISession, reqMsg, respMsg protoadapt.MessageV1) error {
	if session == nil || strings.TrimSpace(session.ID) == "" || len(session.Key) != loaderAPISessionKeySize {
		return fmt.Errorf("loader api session is not ready")
	}

	raw, err := proto.Marshal(protoadapt.MessageV2Of(reqMsg))
	if err != nil {
		return fmt.Errorf("marshal sealed request body: %w", err)
	}
	defer wipeBytes(raw)
	nonce, err := randomBytes(loaderAPINonceSize)
	if err != nil {
		return fmt.Errorf("generate request nonce: %w", err)
	}
	ciphertext, err := sealSessionPayload(session.Key, requestAAD(path, session.ID), nonce, raw)
	if err != nil {
		return fmt.Errorf("seal loader api request: %w", err)
	}

	resp := &loaderapiv1.SealedResponse{}
	if _, err := c.postProto(ctx, httpClient, path, &loaderapiv1.SealedRequest{
		SessionID:  session.ID,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}, resp); err != nil {
		return err
	}

	plaintext, err := openSessionPayload(session.Key, responseAAD(path, session.ID), resp.Nonce, resp.Ciphertext)
	if err != nil {
		return fmt.Errorf("open loader api response: %w", err)
	}
	defer wipeBytes(plaintext)
	if err := proto.Unmarshal(plaintext, protoadapt.MessageV2Of(respMsg)); err != nil {
		return fmt.Errorf("decode sealed protobuf response from %s: %w", path, err)
	}
	return nil
}

func insecureManifestTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
	}
}

func pinnedTLSConfig(manifest *loaderapiv1.ManifestResponse) *tls.Config {
	if manifest == nil {
		return insecureManifestTLSConfig()
	}
	expectedFingerprint := normalizeServerCertFingerprint(manifest.TlsCertSha256)
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("loader api tls peer certificate missing")
			}
			if !fingerprintMatchesCertificate(expectedFingerprint, rawCerts[0]) {
				return fmt.Errorf("loader api tls fingerprint mismatch")
			}
			return nil
		},
	}
}

func requestAAD(path, sessionID string) []byte {
	return []byte("request|" + strings.TrimSpace(sessionID) + "|" + strings.TrimSpace(path))
}

func responseAAD(path, sessionID string) []byte {
	return []byte("response|" + strings.TrimSpace(sessionID) + "|" + strings.TrimSpace(path))
}

func sealSessionPayload(key, aad, nonce, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("invalid nonce length %d", len(nonce))
	}
	return aead.Seal(nil, nonce, plaintext, aad), nil
}

func openSessionPayload(key, aad, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("invalid nonce length %d", len(nonce))
	}
	return aead.Open(nil, nonce, ciphertext, aad)
}

func randomBytes(size int) ([]byte, error) {
	buf := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (c *loaderAPIClient) cachedManifest() (*loaderapiv1.ManifestResponse, bool) {
	loaderAPICache.mu.Lock()
	defer loaderAPICache.mu.Unlock()

	entry, ok := loaderAPICache.manifests[c.baseURL]
	if !ok {
		return nil, false
	}
	if time.Since(entry.CachedAt) > loaderAPIManifestCacheTTL {
		delete(loaderAPICache.manifests, c.baseURL)
		return nil, false
	}
	return cloneManifest(entry.Manifest), true
}

func (c *loaderAPIClient) storeManifest(manifest *loaderapiv1.ManifestResponse) {
	loaderAPICache.mu.Lock()
	defer loaderAPICache.mu.Unlock()
	loaderAPICache.manifests[c.baseURL] = loaderAPIManifestCacheEntry{
		Manifest: cloneManifest(manifest),
		CachedAt: time.Now(),
	}
}

func (c *loaderAPIClient) invalidateManifestCache() {
	loaderAPICache.mu.Lock()
	defer loaderAPICache.mu.Unlock()
	delete(loaderAPICache.manifests, c.baseURL)
}

func (c *loaderAPIClient) cachedSession(fingerprint string) (*loaderAPISession, bool) {
	loaderAPICache.mu.Lock()
	defer loaderAPICache.mu.Unlock()

	entry, ok := loaderAPICache.sessions[c.baseURL]
	if !ok {
		return nil, false
	}
	if entry.Fingerprint != fingerprint {
		wipeLoaderAPISession(entry.Session)
		delete(loaderAPICache.sessions, c.baseURL)
		return nil, false
	}
	if entry.Session == nil || time.Now().After(entry.Session.ExpiresAt) {
		wipeLoaderAPISession(entry.Session)
		delete(loaderAPICache.sessions, c.baseURL)
		return nil, false
	}
	return cloneSession(entry.Session), true
}

func (c *loaderAPIClient) storeSession(fingerprint string, session *loaderAPISession) {
	loaderAPICache.mu.Lock()
	defer loaderAPICache.mu.Unlock()
	if existing, ok := loaderAPICache.sessions[c.baseURL]; ok {
		wipeLoaderAPISession(existing.Session)
	}
	loaderAPICache.sessions[c.baseURL] = loaderAPISessionCacheEntry{
		Session:     cloneSession(session),
		Fingerprint: fingerprint,
	}
}

func (c *loaderAPIClient) invalidateSessionCache() {
	loaderAPICache.mu.Lock()
	defer loaderAPICache.mu.Unlock()
	if existing, ok := loaderAPICache.sessions[c.baseURL]; ok {
		wipeLoaderAPISession(existing.Session)
	}
	delete(loaderAPICache.sessions, c.baseURL)
}

func cloneManifest(manifest *loaderapiv1.ManifestResponse) *loaderapiv1.ManifestResponse {
	if manifest == nil {
		return nil
	}
	cloned := *manifest
	cloned.Signature = append([]byte(nil), manifest.Signature...)
	return &cloned
}

func cloneSession(session *loaderAPISession) *loaderAPISession {
	if session == nil {
		return nil
	}
	return &loaderAPISession{
		ID:        session.ID,
		Key:       append([]byte(nil), session.Key...),
		ExpiresAt: session.ExpiresAt,
	}
}

func wipeLoaderAPISession(session *loaderAPISession) {
	if session == nil {
		return
	}
	wipeBytes(session.Key)
}

func toProtoClientMeta(meta ClientMeta) *loaderapiv1.ClientMeta {
	return &loaderapiv1.ClientMeta{
		ClientName:       strings.TrimSpace(meta.ClientName),
		ClientVersion:    strings.TrimSpace(meta.ClientVersion),
		BuildFingerprint: strings.TrimSpace(meta.BuildFingerprint),
		Platform:         strings.TrimSpace(meta.Platform),
		Locale:           strings.TrimSpace(meta.Locale),
		TransportMode:    strings.TrimSpace(meta.TransportMode),
	}
}

func (ci *clientIdentity) sign(payload []byte) []byte {
	if ci == nil || len(ci.PrivateKey) != ed25519.PrivateKeySize {
		return nil
	}
	return ed25519.Sign(ci.PrivateKey, payload)
}
