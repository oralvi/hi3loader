package config

import (
	"bytes"
	"encoding/json"
	"fmt"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type jsonKind byte

const (
	jsonKindUnknown jsonKind = iota
	jsonKindString
	jsonKindNumber
	jsonKindBool
	jsonKindObject
	jsonKindArray
	jsonKindNull
)

var canonicalFieldKinds = map[string]jsonKind{
	"account":                 jsonKindString,
	"current_account":         jsonKindString,
	"password":                jsonKindString,
	"sleep_time":              jsonKindNumber,
	"clip_check":              jsonKindBool,
	"auto_close":              jsonKindBool,
	"game_path":               jsonKindString,
	"uid":                     jsonKindNumber,
	"access_key":              jsonKindString,
	"HI3UID":                  jsonKindString,
	"BILIHITOKEN":             jsonKindString,
	"asterisk_name":           jsonKindString,
	"last_login_succ":         jsonKindBool,
	"bh_ver":                  jsonKindString,
	"bili_pkg_ver":            jsonKindNumber,
	"uname":                   jsonKindString,
	"accounts":                jsonKindArray,
	"auto_clip":               jsonKindBool,
	"account_login":           jsonKindBool,
	"version_api":             jsonKindString,
	"dispatch_api":            jsonKindString,
	"dispatch_data":           jsonKindString,
	"dispatch_version":        jsonKindString,
	"dispatch_source":         jsonKindString,
	"dispatch_raw_len":        jsonKindNumber,
	"dispatch_decoded_len":    jsonKindNumber,
	"dispatch_decoded_sha256": jsonKindString,
	"dispatch_saved_at":       jsonKindString,
	"background_image":        jsonKindString,
	"background_opacity":      jsonKindNumber,
	"panel_blur":              jsonKindBool,
	"crypto_salt":             jsonKindString,
}

func jsonUnmarshal(data []byte, out any) error {
	data = stripUTF8BOM(data)

	switch target := out.(type) {
	case *storedConfig:
		cfg, legacy, salt, err := decodeLooseConfigObject(data)
		if err != nil {
			return err
		}
		target.Config = *cfg
		target.CryptoSalt = salt
		applyLegacyDispatchCache(&target.Config, legacy)
		return nil
	case *Config:
		cfg, legacy, _, err := decodeLooseConfigObject(data)
		if err != nil {
			return err
		}
		*target = *cfg
		applyLegacyDispatchCache(target, legacy)
		return nil
	}
	return json.Unmarshal(data, out)
}

func decodeLooseConfigObject(data []byte) (*Config, map[string]DispatchCacheEntry, string, error) {
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, "", err
	}

	cfg := Default()
	for key, message := range raw {
		value, ok, err := decodeLooseValue(message)
		if err != nil {
			return nil, nil, "", fmt.Errorf("%s: %w", key, err)
		}
		if !ok {
			continue
		}
		switch key {
		case "account":
			cfg.Account = StringValue(value)
		case "current_account":
			cfg.CurrentAccount = StringValue(value)
		case "password":
			cfg.Password = StringValue(value)
		case "sleep_time":
			cfg.SleepTime = IntValue(value)
		case "clip_check":
			cfg.ClipCheck = BoolValue(value)
		case "auto_close":
			cfg.AutoClose = BoolValue(value)
		case "game_path":
			cfg.GamePath = StringValue(value)
		case "uid":
			cfg.UID = Int64Value(value)
		case "access_key":
			cfg.AccessKey = StringValue(value)
		case "HI3UID":
			cfg.HI3UID = StringValue(value)
		case "BILIHITOKEN":
			cfg.BILIHITOKEN = StringValue(value)
		case "asterisk_name":
			cfg.AsteriskName = StringValue(value)
		case "last_login_succ":
			cfg.LastLoginSucc = BoolValue(value)
		case "bh_ver":
			cfg.BHVer = StringValue(value)
		case "bili_pkg_ver":
			cfg.BiliPkgVer = IntValue(value)
		case "uname":
			cfg.UName = StringValue(value)
		case "accounts":
			cfg.Accounts = decodeLooseSavedAccounts(value)
		case "auto_clip":
			cfg.AutoClip = BoolValue(value)
		case "account_login":
			cfg.AccountLogin = BoolValue(value)
		case "version_api":
			cfg.VersionAPI = StringValue(value)
		case "dispatch_api":
			cfg.DispatchAPI = StringValue(value)
		case "dispatch_data":
			cfg.DispatchData = StringValue(value)
		case "dispatch_version":
			cfg.DispatchVersion = StringValue(value)
		case "dispatch_source":
			cfg.DispatchSource = StringValue(value)
		case "dispatch_raw_len":
			cfg.DispatchRawLen = IntValue(value)
		case "dispatch_decoded_len":
			cfg.DispatchDecodedLen = IntValue(value)
		case "dispatch_decoded_sha256":
			cfg.DispatchDecodedSHA256 = StringValue(value)
		case "dispatch_saved_at":
			cfg.DispatchSavedAt = StringValue(value)
		case "background_image":
			cfg.BackgroundImage = StringValue(value)
		case "background_opacity":
			cfg.BackgroundOpacity = Float64Value(value)
		case "panel_blur":
			cfg.PanelBlur = BoolValue(value)
		}
	}

	var legacy map[string]DispatchCacheEntry
	if message, ok := raw["dispatch_cache"]; ok && !isNullJSON(message) {
		if err := json.Unmarshal(message, &legacy); err != nil {
			return nil, nil, "", fmt.Errorf("dispatch_cache: %w", err)
		}
	}

	salt := ""
	if message, ok := raw["crypto_salt"]; ok && !isNullJSON(message) {
		value, ok, err := decodeLooseValue(message)
		if err != nil {
			return nil, nil, "", fmt.Errorf("crypto_salt: %w", err)
		}
		if ok {
			salt = StringValue(value)
		}
	}

	return cfg, legacy, salt, nil
}

func decodeLooseSavedAccounts(value any) []SavedAccount {
	rawEntries, ok := value.([]any)
	if !ok {
		return nil
	}

	accounts := make([]SavedAccount, 0, len(rawEntries))
	for _, rawEntry := range rawEntries {
		entryMap, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		account := SavedAccount{
			Account:       StringValue(entryMap["account"]),
			Password:      StringValue(entryMap["password"]),
			UID:           Int64Value(entryMap["uid"]),
			AccessKey:     StringValue(entryMap["access_key"]),
			UName:         StringValue(entryMap["uname"]),
			LastLoginSucc: BoolValue(entryMap["last_login_succ"]),
		}
		if !normalizeSavedAccount(&account) {
			continue
		}
		accounts = append(accounts, account)
	}
	if len(accounts) == 0 {
		return nil
	}
	return accounts
}

func decodeLooseValue(raw json.RawMessage) (any, bool, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, false, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func stripUTF8BOM(data []byte) []byte {
	return bytes.TrimPrefix(data, utf8BOM)
}

func isNullJSON(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func jsonValueKindOf(raw json.RawMessage) jsonKind {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return jsonKindUnknown
	}
	switch trimmed[0] {
	case '"':
		return jsonKindString
	case '{':
		return jsonKindObject
	case '[':
		return jsonKindArray
	case 't', 'f':
		return jsonKindBool
	case 'n':
		return jsonKindNull
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return jsonKindNumber
	default:
		return jsonKindUnknown
	}
}
