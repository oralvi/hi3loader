package service

import (
	"fmt"
	"strings"

	"hi3loader/internal/config"
)

var alphaLegacyCaptchaDeviceProfile = config.DeviceProfile{
	PresetID:        "alpha-legacy-mumu",
	Model:           "MuMu",
	Brand:           "Android",
	SupportABIs:     "arm64-v8a,armeabi-v7a,armeabi",
	Display:         "1280*720",
	AndroidID:       "84567e2dda72d1d4",
	MACAddress:      "08:00:27:53:DD:12",
	IMEI:            "227656364311444",
	RuntimeUDID:     "KREhESMUIhUjFnJKNko2TDQFYlZkB3cdeQ==",
	UserProfileUDID: "XXA31CBAB6CBA63E432E087B58411A213BFB7",
	CurBuvid:        "XZA2FA4AC240F665E2F27F603ABF98C615C29",
}

func (s *Service) EnableLegacyCaptchaLoginMode(account string) error {
	if s == nil {
		return nil
	}

	account = strings.TrimSpace(account)

	s.mu.Lock()
	if s.cfg == nil {
		s.mu.Unlock()
		return fmt.Errorf("config is not loaded")
	}

	nextCfg := s.cfg.Clone()
	nextCfg.AccountLogin = false
	nextCfg.DeviceProfile = alphaLegacyCaptchaDeviceProfile

	if account != "" {
		entry, ok := nextCfg.FindSavedAccount(account)
		if !ok {
			entry = config.SavedAccount{Account: account}
		}
		entry.Account = account
		entry.AccessKey = ""
		entry.LastLoginSucc = false
		entry.DeviceProfile = alphaLegacyCaptchaDeviceProfile
		nextCfg.UpsertSavedAccount(entry)
		nextCfg.CurrentAccount = account
	} else if current, ok := nextCfg.CurrentSavedAccount(); ok {
		current.AccessKey = ""
		current.LastLoginSucc = false
		current.DeviceProfile = alphaLegacyCaptchaDeviceProfile
		nextCfg.UpsertSavedAccount(current)
	}

	if err := config.Save(s.cfgPath, nextCfg); err != nil {
		s.mu.Unlock()
		return err
	}
	s.cfg = nextCfg
	s.mu.Unlock()
	s.logf("alpha login test enabled legacy device profile for %s", maskSecret(account))
	return nil
}
