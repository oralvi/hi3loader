package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	envStoreMagic           = "ENV1"
	defaultMasterKeyEnv     = "HI3LOADER_MASTER_KEY"
	defaultMasterKeyFileEnv = "HI3LOADER_MASTER_KEY_FILE"
)

type EnvStore struct {
	key []byte
}

func NewEnvStore(masterKey []byte) (*EnvStore, error) {
	switch len(masterKey) {
	case 16, 24, 32:
	default:
		return nil, fmt.Errorf("env store requires a 16, 24, or 32 byte key")
	}
	return &EnvStore{key: append([]byte(nil), masterKey...)}, nil
}

func NewEnvStoreFromEnv() (*EnvStore, error) {
	if raw := strings.TrimSpace(os.Getenv(defaultMasterKeyEnv)); raw != "" {
		key, err := decodeMasterKey(raw)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", defaultMasterKeyEnv, err)
		}
		return NewEnvStore(key)
	}
	if path := strings.TrimSpace(os.Getenv(defaultMasterKeyFileEnv)); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", defaultMasterKeyFileEnv, err)
		}
		key, err := decodeMasterKey(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("decode %s contents: %w", defaultMasterKeyFileEnv, err)
		}
		return NewEnvStore(key)
	}
	return nil, fmt.Errorf("missing %s or %s", defaultMasterKeyEnv, defaultMasterKeyFileEnv)
}

func (s *EnvStore) Protect(plaintext []byte) ([]byte, error) {
	if s == nil || len(s.key) == 0 {
		return nil, fmt.Errorf("env store is closed")
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(envStoreMagic)+len(nonce)+len(plaintext)+aead.Overhead())
	out = append(out, envStoreMagic...)
	out = append(out, nonce...)
	out = aead.Seal(out, nonce, plaintext, nil)
	return out, nil
}

func (s *EnvStore) Unprotect(ciphertext []byte) ([]byte, error) {
	if s == nil || len(s.key) == 0 {
		return nil, fmt.Errorf("env store is closed")
	}
	if len(ciphertext) < len(envStoreMagic) || string(ciphertext[:len(envStoreMagic)]) != envStoreMagic {
		return nil, fmt.Errorf("invalid env store payload")
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	offset := len(envStoreMagic)
	if len(ciphertext) < offset+aead.NonceSize() {
		return nil, fmt.Errorf("truncated env store payload")
	}
	nonce := ciphertext[offset : offset+aead.NonceSize()]
	blob := ciphertext[offset+aead.NonceSize():]
	return aead.Open(nil, nonce, blob, nil)
}

func (s *EnvStore) Close() error {
	if s == nil {
		return nil
	}
	wipeBytes(s.key)
	s.key = nil
	return nil
}

func decodeMasterKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("master key is empty")
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(raw); err == nil {
		return decoded, nil
	}
	return []byte(raw), nil
}
