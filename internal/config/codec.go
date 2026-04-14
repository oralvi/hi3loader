package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"hi3loader/internal/secrets"
)

type Codec struct {
	secretStore secrets.SecretStore
}

func NewCodec(store secrets.SecretStore) (*Codec, error) {
	if store == nil {
		return nil, fmt.Errorf("config codec requires a secret store")
	}
	return &Codec{secretStore: store}, nil
}

func NewDefaultCodec() (*Codec, error) {
	store, err := secrets.NewDefaultStore()
	if err != nil {
		return nil, err
	}
	return NewCodec(store)
}

func (c *Codec) Close() error {
	if c == nil || c.secretStore == nil {
		return nil
	}
	return c.secretStore.Close()
}

func wipeBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

func (c *Codec) LoadOrCreate(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := Default()
		cfg.Normalize()
		if err := c.Save(path, cfg); err != nil {
			return nil, err
		}
		cfg.AccountLogin = false
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg, migrated, err := c.decodeStoredConfig(data)
	if err != nil {
		if backupErr := backupCorruptConfig(path, data); backupErr != nil {
			return nil, fmt.Errorf("backup corrupt config: %w", backupErr)
		}
		if cfg == nil {
			cfg = Default()
		}
		cfg.Normalize()
		if err := c.Save(path, cfg); err != nil {
			return nil, err
		}
		cfg.AccountLogin = false
		return cfg, nil
	}
	cfg.AccountLogin = false
	if cfg.Normalize() || migrated {
		if err := c.Save(path, cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func (c *Codec) Save(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("save config: config is nil")
	}
	cfg.Normalize()
	stored, err := c.encodeStoredConfig(cfg)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := AtomicWriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
