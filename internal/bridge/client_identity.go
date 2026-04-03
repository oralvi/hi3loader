package bridge

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"hi3loader/internal/loaderapiv1"
)

const clientIdentityFileName = "module.identity"

type clientIdentity struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

type clientIdentityEnvelope struct {
	PublicKey  []byte `json:"public_key"`
	PrivateKey []byte `json:"private_key"`
}

var (
	clientIdentityOnce sync.Once
	clientIdentityInst *clientIdentity
	clientIdentityErr  error
)

func loadClientIdentity() (*clientIdentity, error) {
	clientIdentityOnce.Do(func() {
		clientIdentityInst, clientIdentityErr = loadOrCreateClientIdentity()
	})
	return clientIdentityInst, clientIdentityErr
}

func loadOrCreateClientIdentity() (*clientIdentity, error) {
	path, err := clientIdentityPath()
	if err != nil {
		return nil, err
	}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		plaintext, err := unprotectClientIdentity(data)
		if err != nil {
			return nil, fmt.Errorf("unprotect client identity: %w", err)
		}
		identity, err := decodeClientIdentity(plaintext)
		if err != nil {
			return nil, err
		}
		return identity, nil
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate client identity: %w", err)
	}
	identity := &clientIdentity{
		PublicKey:  append(ed25519.PublicKey(nil), publicKey...),
		PrivateKey: append(ed25519.PrivateKey(nil), privateKey...),
	}
	plaintext, err := json.Marshal(clientIdentityEnvelope{
		PublicKey:  identity.PublicKey,
		PrivateKey: identity.PrivateKey,
	})
	if err != nil {
		return nil, fmt.Errorf("encode client identity: %w", err)
	}
	protected, err := protectClientIdentity(plaintext)
	if err != nil {
		return nil, fmt.Errorf("protect client identity: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create identity dir: %w", err)
	}
	if err := os.WriteFile(path, protected, 0o600); err != nil {
		return nil, fmt.Errorf("write client identity: %w", err)
	}
	return identity, nil
}

func decodeClientIdentity(data []byte) (*clientIdentity, error) {
	var envelope clientIdentityEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode client identity: %w", err)
	}
	if len(envelope.PublicKey) != ed25519.PublicKeySize || len(envelope.PrivateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid client identity length")
	}
	return &clientIdentity{
		PublicKey:  append(ed25519.PublicKey(nil), envelope.PublicKey...),
		PrivateKey: append(ed25519.PrivateKey(nil), envelope.PrivateKey...),
	}, nil
}

func (ci *clientIdentity) keyID() string {
	if ci == nil || len(ci.PublicKey) == 0 {
		return ""
	}
	sum := sha256.Sum256(ci.PublicKey)
	return hex.EncodeToString(sum[:8])
}

func clientIdentityPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return filepath.Join(filepath.Dir(exePath), clientIdentityFileName), nil
}

func handshakeSignaturePayload(req *loaderapiv1.HandshakeRequest) []byte {
	if req == nil {
		return nil
	}
	clientName := ""
	clientVersion := ""
	buildFingerprint := ""
	platform := ""
	locale := ""
	transportMode := ""
	if req.ClientMeta != nil {
		clientName = strings.TrimSpace(req.ClientMeta.ClientName)
		clientVersion = strings.TrimSpace(req.ClientMeta.ClientVersion)
		buildFingerprint = strings.TrimSpace(req.ClientMeta.BuildFingerprint)
		platform = strings.TrimSpace(req.ClientMeta.Platform)
		locale = strings.TrimSpace(req.ClientMeta.Locale)
		transportMode = strings.TrimSpace(req.ClientMeta.TransportMode)
	}
	parts := []string{
		clientName,
		clientVersion,
		buildFingerprint,
		platform,
		locale,
		transportMode,
		hex.EncodeToString(req.ClientPublicKey),
		hex.EncodeToString(req.ClientNonce),
		fmt.Sprintf("%d", req.UnixMs),
		hex.EncodeToString(req.ClientIdentityKey),
		strings.TrimSpace(req.ClientIdentityKeyId),
	}
	return []byte(strings.Join(parts, "\n"))
}
