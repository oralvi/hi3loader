package config

import "encoding/json"

func jsonUnmarshal(data []byte, out any) error {
	switch target := out.(type) {
	case *storedConfig:
		type storedAlias storedConfig
		type compatStored struct {
			storedAlias
			DispatchCache map[string]DispatchCacheEntry `json:"dispatch_cache,omitempty"`
		}

		raw := compatStored{
			storedAlias: storedAlias{
				Config: *Default(),
			},
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		*target = storedConfig(raw.storedAlias)
		applyLegacyDispatchCache(&target.Config, raw.DispatchCache)
		target.Config.Normalize()
		return nil
	case *Config:
		type configAlias Config
		type compatConfig struct {
			configAlias
			DispatchCache map[string]DispatchCacheEntry `json:"dispatch_cache,omitempty"`
		}

		raw := compatConfig{
			configAlias: configAlias(*Default()),
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		*target = Config(raw.configAlias)
		applyLegacyDispatchCache(target, raw.DispatchCache)
		target.Normalize()
		return nil
	}
	return json.Unmarshal(data, out)
}
