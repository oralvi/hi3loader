package bsgamesdk

import (
	"strconv"
	"strings"
)

const (
	sdkCurBuvid = "XZA2FA4AC240F665E2F27F603ABF98C615C29"
	sdkMerchantID  = "777"
	sdkSupportABIs = "arm64-v8a,armeabi-v7a,armeabi"
	sdkAPKSign      = "4502a02a00395dec05a4134ad593224d"
	sdkPlatformType = "3"
	sdkOperators    = "5"
	sdkModel        = "MuMu"
	sdkBrand        = "Android"
	sdkGameID = "180"
	sdkAppID   = "180"
	sdkVersion = "1"
	sdkGameVer          = "6.1.0"
	sdkChannelID        = "1"
	sdkChannelIDNumeric = 1
	sdkVersionCode       = "510"
	sdkServerID          = "378"
	sdkDomainSwitchCount = "0"
	sdkPFVer             = "12"
	sdkSDKLogType        = "1"
	sdkSDKVer          = "5.9.0"
	sdkDomain          = "line1-sdk-center-login-sh.biligame.net"
	sdkDisplay         = "1280*720"
	sdkAndroidID       = "84567e2dda72d1d4"
	sdkMACAddress      = "08:00:27:53:DD:12"
	sdkRuntimeUDID     = "KREhESMUIhUjFnJKNko2TDQFYlZkB3cdeQ=="
	sdkUserProfileUDID = "XXA31CBAB6CBA63E432E087B58411A213BFB7"
	sdkNetUserProfile  = "5"
	sdkNetDefault      = "4"
	sdkCaptchaType     = "1"
)

type payloadField struct {
	key   string
	value any
}

type RuntimeProfile struct {
	ChannelID      int64
	AppID          int64
	CPID           string
	CPAppID        string
	CPAppKey       string
	ServerID       int64
	ChannelVersion string
	GameVer        string
	VersionCode    int64
	SDKVer         string
}

type DeviceProfile struct {
	AndroidID       string
	MACAddress      string
	IMEI            string
	RuntimeUDID     string
	UserProfileUDID string
	CurBuvid        string
}

type effectiveSDKProfile struct {
	CurBuvid          string
	MerchantID        string
	SupportABIs       string
	APKSign           string
	PlatformType      string
	Operators         string
	Model             string
	Brand             string
	GameID            string
	AppID             string
	Version           string
	GameVer           string
	ChannelID         string
	ChannelIDNumeric  int
	VersionCode       string
	ServerID          string
	DomainSwitchCount string
	PFVer             string
	SDKLogType        string
	SDKVer            string
	Domain            string
	Display           string
	AndroidID         string
	MACAddress        string
	IMEI              string
	RuntimeUDID       string
	UserProfileUDID   string
	NetUserProfile    string
	NetDefault        string
	CaptchaType       string
	SignatureMaterial string
}

func newPayloadTemplate(fields ...payloadField) payloadTemplate {
	data := make(map[string]any, len(fields))
	order := make([]string, 0, len(fields))
	for _, field := range fields {
		data[field.key] = field.value
		order = append(order, field.key)
	}
	return payloadTemplate{
		data:  data,
		order: order,
	}
}

func userProfileTemplate(profile effectiveSDKProfile) payloadTemplate {
	return newPayloadTemplate(
		payloadField{"cur_buvid", profile.CurBuvid},
		payloadField{"client_timestamp", ""},
		payloadField{"sdk_type", "1"},
		payloadField{"isRoot", "0"},
		payloadField{"merchant_id", profile.MerchantID},
		payloadField{"dp", profile.Display},
		payloadField{"mac", profile.MACAddress},
		payloadField{"uid", ""},
		payloadField{"support_abis", profile.SupportABIs},
		payloadField{"apk_sign", profile.APKSign},
		payloadField{"platform_type", profile.PlatformType},
		payloadField{"old_buvid", profile.CurBuvid},
		payloadField{"operators", profile.Operators},
		payloadField{"fingerprint", ""},
		payloadField{"model", profile.Model},
		payloadField{"udid", profile.UserProfileUDID},
		payloadField{"net", profile.NetUserProfile},
		payloadField{"app_id", profile.AppID},
		payloadField{"brand", profile.Brand},
		payloadField{"oaid", ""},
		payloadField{"game_id", profile.GameID},
		payloadField{"timestamp", ""},
		payloadField{"ver", profile.GameVer},
		payloadField{"c", "1"},
		payloadField{"version_code", profile.VersionCode},
		payloadField{"server_id", profile.ServerID},
		payloadField{"version", profile.Version},
		payloadField{"domain_switch_count", profile.DomainSwitchCount},
		payloadField{"pf_ver", profile.PFVer},
		payloadField{"access_key", ""},
		payloadField{"domain", profile.Domain},
		payloadField{"original_domain", ""},
		payloadField{"imei", profile.IMEI},
		payloadField{"sdk_log_type", profile.SDKLogType},
		payloadField{"sdk_ver", profile.SDKVer},
		payloadField{"android_id", profile.AndroidID},
		payloadField{"channel_id", profile.ChannelIDNumeric},
	)
}

func keyTemplate(profile effectiveSDKProfile) payloadTemplate {
	fields := append(commonRuntimeFields(profile, profile.NetDefault),
		payloadField{"uid", ""},
		payloadField{"game_id", profile.GameID},
		payloadField{"ver", profile.GameVer},
		payloadField{"model", profile.Model},
	)
	return newPayloadTemplate(fields...)
}

func challengeTemplate(profile effectiveSDKProfile) payloadTemplate {
	fields := append(commonRuntimeFields(profile, profile.NetDefault),
		payloadField{"uid", ""},
		payloadField{"game_id", profile.GameID},
		payloadField{"ver", profile.GameVer},
		payloadField{"model", profile.Model},
	)
	return newPayloadTemplate(fields...)
}

func credentialTemplate(profile effectiveSDKProfile) payloadTemplate {
	fields := append(commonRuntimeFields(profile, profile.NetDefault),
		payloadField{"gt_user_id", ""},
		payloadField{"seccode", ""},
		payloadField{"validate", ""},
		payloadField{"pwd", ""},
		payloadField{"uid", ""},
		payloadField{"captcha_type", profile.CaptchaType},
		payloadField{"game_id", profile.GameID},
		payloadField{"challenge", ""},
		payloadField{"user_id", ""},
		payloadField{"ver", profile.GameVer},
		payloadField{"model", profile.Model},
	)
	return newPayloadTemplate(fields...)
}

func commonRuntimeFields(profile effectiveSDKProfile, net string) []payloadField {
	return []payloadField{
		{"operators", profile.Operators},
		{"merchant_id", profile.MerchantID},
		{"isRoot", "0"},
		{"domain_switch_count", profile.DomainSwitchCount},
		{"sdk_type", "1"},
		{"sdk_log_type", profile.SDKLogType},
		{"timestamp", ""},
		{"support_abis", profile.SupportABIs},
		{"access_key", ""},
		{"sdk_ver", profile.SDKVer},
		{"oaid", ""},
		{"dp", profile.Display},
		{"original_domain", ""},
		{"imei", profile.IMEI},
		{"version", profile.Version},
		{"udid", profile.RuntimeUDID},
		{"apk_sign", profile.APKSign},
		{"platform_type", profile.PlatformType},
		{"old_buvid", profile.CurBuvid},
		{"android_id", profile.AndroidID},
		{"fingerprint", ""},
		{"mac", profile.MACAddress},
		{"server_id", profile.ServerID},
		{"domain", profile.Domain},
		{"app_id", profile.AppID},
		{"version_code", profile.VersionCode},
		{"net", net},
		{"pf_ver", profile.PFVer},
		{"cur_buvid", profile.CurBuvid},
		{"c", "1"},
		{"brand", profile.Brand},
		{"client_timestamp", ""},
		{"channel_id", profile.ChannelID},
	}
}

func buildEffectiveSDKProfile(runtime RuntimeProfile, device DeviceProfile) effectiveSDKProfile {
	channelID := normalizedIntString(runtime.ChannelID, sdkChannelID)
	channelNumeric := normalizedNumericInt(channelID, sdkChannelIDNumeric)
	return effectiveSDKProfile{
		CurBuvid:          fallbackString(device.CurBuvid, sdkCurBuvid),
		MerchantID:        fallbackString(runtime.CPID, sdkMerchantID),
		SupportABIs:       sdkSupportABIs,
		APKSign:           sdkAPKSign,
		PlatformType:      sdkPlatformType,
		Operators:         sdkOperators,
		Model:             sdkModel,
		Brand:             sdkBrand,
		GameID:            sdkGameID,
		AppID:             sdkAppID,
		Version:           sdkVersion,
		GameVer:           fallbackString(runtime.GameVer, sdkGameVer),
		ChannelID:         channelID,
		ChannelIDNumeric:  channelNumeric,
		VersionCode:       normalizedIntString(runtime.VersionCode, sdkVersionCode),
		ServerID:          normalizedIntString(runtime.ServerID, sdkServerID),
		DomainSwitchCount: sdkDomainSwitchCount,
		PFVer:             sdkPFVer,
		SDKLogType:        sdkSDKLogType,
		SDKVer:            fallbackString(runtime.SDKVer, sdkSDKVer),
		Domain:            sdkDomain,
		Display:           sdkDisplay,
		AndroidID:         fallbackString(device.AndroidID, sdkAndroidID),
		MACAddress:        fallbackString(device.MACAddress, sdkMACAddress),
		RuntimeUDID:       fallbackString(device.RuntimeUDID, sdkRuntimeUDID),
		UserProfileUDID:   fallbackString(device.UserProfileUDID, sdkUserProfileUDID),
		NetUserProfile:    sdkNetUserProfile,
		NetDefault:        sdkNetDefault,
		CaptchaType:       sdkCaptchaType,
		IMEI:              fallbackString(device.IMEI, ""),
		SignatureMaterial: fallbackString(runtime.CPAppKey, signatureMaterial()),
	}
}

func fallbackString(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func normalizedIntString(value int64, fallback string) string {
	if value > 0 {
		return strconv.FormatInt(value, 10)
	}
	return fallback
}

func normalizedNumericString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return value
	}
	return fallback
}

func normalizedNumericInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
