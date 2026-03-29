package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	storageEnvelopePrefix = "enc:v1:"
	storageSecretEnvVar   = "HI3LOADER_SECRET_DIR"
	machineIDEnvVar       = "HI3LOADER_MACHINE_ID"
	storageSecretFileName = "storage.key"
	storageSaltBytes      = 16
)

type storedConfig struct {
	Config
	CryptoSalt string `json:"crypto_salt,omitempty"`
}

type storedConfigJSON struct {
	CurrentAccount        string         `json:"current_account,omitempty"`
	SleepTime             int            `json:"sleep_time"`
	ClipCheck             bool           `json:"clip_check"`
	AutoClose             bool           `json:"auto_close"`
	GamePath              string         `json:"game_path,omitempty"`
	HI3UID                string         `json:"HI3UID,omitempty"`
	BILIHITOKEN           string         `json:"BILIHITOKEN,omitempty"`
	AsteriskName          string         `json:"asterisk_name,omitempty"`
	BHVer                 string         `json:"bh_ver,omitempty"`
	BiliPkgVer            int            `json:"bili_pkg_ver,omitempty"`
	Accounts              []SavedAccount `json:"accounts,omitempty"`
	AutoClip              bool           `json:"auto_clip"`
	VersionAPI            string         `json:"version_api,omitempty"`
	DispatchAPI           string         `json:"dispatch_api,omitempty"`
	DispatchData          string         `json:"dispatch_data,omitempty"`
	DispatchVersion       string         `json:"dispatch_version,omitempty"`
	DispatchSource        string         `json:"dispatch_source,omitempty"`
	DispatchRawLen        int            `json:"dispatch_raw_len,omitempty"`
	DispatchDecodedLen    int            `json:"dispatch_decoded_len,omitempty"`
	DispatchDecodedSHA256 string         `json:"dispatch_decoded_sha256,omitempty"`
	DispatchSavedAt       string         `json:"dispatch_saved_at,omitempty"`
	BackgroundImage       string         `json:"background_image,omitempty"`
	BackgroundOpacity     float64        `json:"background_opacity"`
	PanelBlur             bool           `json:"panel_blur"`
	CryptoSalt            string         `json:"crypto_salt,omitempty"`
}

type configCipher struct {
	aead cipher.AEAD
}

var (
	deviceSecretOnce sync.Once
	deviceSecretData []byte
	deviceSecretErr  error
)

func (s storedConfig) MarshalJSON() ([]byte, error) {
	payload := storedConfigJSON{
		CurrentAccount:        normalizeString(s.CurrentAccount),
		SleepTime:             s.SleepTime,
		ClipCheck:             s.ClipCheck,
		AutoClose:             s.AutoClose,
		GamePath:              normalizeString(s.GamePath),
		HI3UID:                normalizeString(s.HI3UID),
		BILIHITOKEN:           normalizeString(s.BILIHITOKEN),
		AsteriskName:          normalizeString(s.AsteriskName),
		BHVer:                 normalizeString(s.BHVer),
		BiliPkgVer:            s.BiliPkgVer,
		Accounts:              append([]SavedAccount(nil), s.Accounts...),
		AutoClip:              s.AutoClip,
		VersionAPI:            normalizeString(s.VersionAPI),
		DispatchAPI:           normalizeString(s.DispatchAPI),
		DispatchData:          normalizeString(s.DispatchData),
		DispatchVersion:       normalizeString(s.DispatchVersion),
		DispatchSource:        normalizeString(s.DispatchSource),
		DispatchRawLen:        s.DispatchRawLen,
		DispatchDecodedLen:    s.DispatchDecodedLen,
		DispatchDecodedSHA256: normalizeString(s.DispatchDecodedSHA256),
		DispatchSavedAt:       normalizeString(s.DispatchSavedAt),
		BackgroundImage:       normalizeString(s.BackgroundImage),
		BackgroundOpacity:     s.BackgroundOpacity,
		PanelBlur:             s.PanelBlur,
		CryptoSalt:            normalizeString(s.CryptoSalt),
	}
	return json.Marshal(payload)
}

func encodeStoredConfig(cfg *Config) (*storedConfig, error) {
	clone := cfg.Clone()
	if clone == nil {
		return nil, fmt.Errorf("encode config: config is nil")
	}
	if clone.CurrentAccount == "" {
		clone.CurrentAccount = strings.TrimSpace(clone.Account)
	}
	clone.Account = ""
	clone.Password = ""
	clone.UID = 0
	clone.AccessKey = ""
	clone.UName = ""
	clone.LastLoginSucc = false
	clone.AccountLogin = false
	if !needsSensitiveStorage(clone) {
		clone.cryptoSalt = ""
		cfg.cryptoSalt = ""
	} else if clone.cryptoSalt == "" {
		salt, err := randomEncodedBytes(storageSaltBytes)
		if err != nil {
			return nil, fmt.Errorf("generate crypto salt: %w", err)
		}
		clone.cryptoSalt = salt
		cfg.cryptoSalt = salt
	}

	if err := encryptSensitiveFields(clone); err != nil {
		return nil, err
	}

	return &storedConfig{
		Config:     *clone,
		CryptoSalt: clone.cryptoSalt,
	}, nil
}

func decodeStoredConfig(raw []byte) (*Config, bool, error) {
	stored := storedConfig{
		Config: *Default(),
	}
	if err := jsonUnmarshal(raw, &stored); err != nil {
		return nil, false, fmt.Errorf("decode config: %w", err)
	}

	cfg := &stored.Config
	cfg.cryptoSalt = normalizeString(stored.CryptoSalt)

	storageChanged, err := decryptSensitiveFields(cfg)
	if err != nil {
		clearSensitiveFields(cfg)
		cfg.Normalize()
		return cfg, true, fmt.Errorf("decrypt config: %w", err)
	}
	return cfg, storageChanged, nil
}

func encryptSensitiveFields(cfg *Config) error {
	if cfg == nil || !needsSensitiveStorage(cfg) {
		return nil
	}
	crypt, err := newConfigCipher(cfg.cryptoSalt)
	if err != nil {
		return err
	}

	if cfg.Password, err = crypt.sealString("password", cfg.Password); err != nil {
		return err
	}
	if cfg.AccessKey, err = crypt.sealString("access_key", cfg.AccessKey); err != nil {
		return err
	}
	if cfg.BILIHITOKEN, err = crypt.sealString("bili_hitoken", cfg.BILIHITOKEN); err != nil {
		return err
	}
	if cfg.DispatchData, err = crypt.sealString("dispatch_data", cfg.DispatchData); err != nil {
		return err
	}
	for idx := range cfg.Accounts {
		label := fmt.Sprintf("accounts[%d]", idx)
		if cfg.Accounts[idx].Password, err = crypt.sealString(label+".password", cfg.Accounts[idx].Password); err != nil {
			return err
		}
		if cfg.Accounts[idx].AccessKey, err = crypt.sealString(label+".access_key", cfg.Accounts[idx].AccessKey); err != nil {
			return err
		}
	}
	return nil
}

func decryptSensitiveFields(cfg *Config) (bool, error) {
	if cfg == nil {
		return false, nil
	}

	needsCipher := hasEncryptedSensitiveValue(cfg)
	if needsCipher && cfg.cryptoSalt == "" {
		return false, fmt.Errorf("missing crypto salt")
	}

	var (
		crypt          *configCipher
		err            error
		storageChanged bool
	)
	if needsCipher {
		crypt, err = newConfigCipher(cfg.cryptoSalt)
		if err != nil {
			return false, err
		}
	}

	if cfg.Password, storageChanged, err = decryptMaybeEncryptedString(crypt, "password", cfg.Password, storageChanged); err != nil {
		return storageChanged, err
	}
	if cfg.AccessKey, storageChanged, err = decryptMaybeEncryptedString(crypt, "access_key", cfg.AccessKey, storageChanged); err != nil {
		return storageChanged, err
	}
	if cfg.BILIHITOKEN, storageChanged, err = decryptMaybeEncryptedString(crypt, "bili_hitoken", cfg.BILIHITOKEN, storageChanged); err != nil {
		return storageChanged, err
	}
	if cfg.DispatchData, storageChanged, err = decryptMaybeEncryptedString(crypt, "dispatch_data", cfg.DispatchData, storageChanged); err != nil {
		return storageChanged, err
	}
	for idx := range cfg.Accounts {
		label := fmt.Sprintf("accounts[%d]", idx)
		if cfg.Accounts[idx].Password, storageChanged, err = decryptMaybeEncryptedString(crypt, label+".password", cfg.Accounts[idx].Password, storageChanged); err != nil {
			return storageChanged, err
		}
		if cfg.Accounts[idx].AccessKey, storageChanged, err = decryptMaybeEncryptedString(crypt, label+".access_key", cfg.Accounts[idx].AccessKey, storageChanged); err != nil {
			return storageChanged, err
		}
	}
	return storageChanged, nil
}

func decryptMaybeEncryptedString(crypt *configCipher, fieldName, value string, storageChanged bool) (string, bool, error) {
	value = normalizeString(value)
	if value == "" {
		return "", storageChanged, nil
	}
	if !strings.HasPrefix(value, storageEnvelopePrefix) {
		return value, true, nil
	}
	if crypt == nil {
		return "", storageChanged, fmt.Errorf("missing cipher for %s", fieldName)
	}
	plain, err := crypt.openString(fieldName, value)
	if err != nil {
		return "", storageChanged, fmt.Errorf("%s: %w", fieldName, err)
	}
	return plain, storageChanged, nil
}

func clearSensitiveFields(cfg *Config) {
	if cfg == nil {
		return
	}
	cfg.Password = ""
	cfg.AccessKey = ""
	cfg.BILIHITOKEN = ""
	cfg.DispatchData = ""
	for idx := range cfg.Accounts {
		cfg.Accounts[idx].Password = ""
		cfg.Accounts[idx].AccessKey = ""
		cfg.Accounts[idx].LastLoginSucc = false
	}
	cfg.DispatchVersion = ""
	cfg.DispatchSource = ""
	cfg.DispatchRawLen = 0
	cfg.DispatchDecodedLen = 0
	cfg.DispatchDecodedSHA256 = ""
	cfg.DispatchSavedAt = ""
	cfg.cryptoSalt = ""
}

func hasEncryptedSensitiveValue(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	if isEncryptedString(cfg.Password) || isEncryptedString(cfg.AccessKey) || isEncryptedString(cfg.BILIHITOKEN) || isEncryptedString(cfg.DispatchData) {
		return true
	}
	for _, account := range cfg.Accounts {
		if isEncryptedString(account.Password) || isEncryptedString(account.AccessKey) {
			return true
		}
	}
	return false
}

func needsSensitiveStorage(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	if normalizeString(cfg.Password) != "" || normalizeString(cfg.AccessKey) != "" || normalizeString(cfg.BILIHITOKEN) != "" || normalizeString(cfg.DispatchData) != "" {
		return true
	}
	for _, account := range cfg.Accounts {
		if normalizeString(account.Password) != "" || normalizeString(account.AccessKey) != "" {
			return true
		}
	}
	return false
}

func isEncryptedString(value string) bool {
	return strings.HasPrefix(normalizeString(value), storageEnvelopePrefix)
}

func newConfigCipher(salt string) (*configCipher, error) {
	salt = normalizeString(salt)
	if salt == "" {
		return nil, fmt.Errorf("crypto salt is empty")
	}
	key, err := deriveConfigKey(salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &configCipher{aead: aead}, nil
}

func (c *configCipher) sealString(fieldName, value string) (string, error) {
	value = normalizeString(value)
	if value == "" {
		return "", nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, []byte(value), []byte(fieldName))
	payload := append(nonce, sealed...)
	return storageEnvelopePrefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

func (c *configCipher) openString(fieldName, value string) (string, error) {
	encoded := strings.TrimPrefix(normalizeString(value), storageEnvelopePrefix)
	payload, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode payload: %w", err)
	}
	nonceSize := c.aead.NonceSize()
	if len(payload) < nonceSize {
		return "", fmt.Errorf("payload is too short")
	}
	nonce := payload[:nonceSize]
	ciphertext := payload[nonceSize:]
	plain, err := c.aead.Open(nil, nonce, ciphertext, []byte(fieldName))
	if err != nil {
		return "", fmt.Errorf("open payload: %w", err)
	}
	return string(plain), nil
}

func deriveConfigKey(salt string) ([]byte, error) {
	deviceSecret, err := loadOrCreateDeviceSecret()
	if err != nil {
		return nil, fmt.Errorf("load device secret: %w", err)
	}
	machineID, err := loadMachineID()
	if err != nil {
		machineID = ""
	}
	sum := sha256.Sum256([]byte("hi3loader/config/v1\x00" + machineID + "\x00" + salt + "\x00" + base64.RawStdEncoding.EncodeToString(deviceSecret)))
	key := make([]byte, len(sum))
	copy(key, sum[:])
	return key, nil
}

func loadOrCreateDeviceSecret() ([]byte, error) {
	deviceSecretOnce.Do(func() {
		deviceSecretData, deviceSecretErr = readOrCreateDeviceSecret()
	})
	if deviceSecretErr != nil {
		return nil, deviceSecretErr
	}
	cloned := make([]byte, len(deviceSecretData))
	copy(cloned, deviceSecretData)
	return cloned, nil
}

func readOrCreateDeviceSecret() ([]byte, error) {
	dir, err := storageSecretDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create secret dir: %w", err)
	}
	path := filepath.Join(dir, storageSecretFileName)
	if data, err := os.ReadFile(path); err == nil {
		if len(data) >= 32 {
			secret := make([]byte, 32)
			copy(secret, data[:32])
			return secret, nil
		}
		return nil, fmt.Errorf("device secret file is invalid")
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read device secret: %w", err)
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate device secret: %w", err)
	}
	if err := atomicWriteFile(path, secret, 0o600); err != nil {
		return nil, fmt.Errorf("write device secret: %w", err)
	}
	return secret, nil
}

func storageSecretDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv(storageSecretEnvVar)); override != "" {
		return override, nil
	}
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(baseDir, "hi3loader"), nil
}

func backupCorruptConfig(path string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	backupPath := fmt.Sprintf("%s.corrupt-%s", path, time.Now().Format("20060102-150405"))
	return atomicWriteFile(backupPath, data, 0o600)
}

func randomEncodedBytes(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(buf), nil
}
