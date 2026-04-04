package config

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

type DevicePreset struct {
	ID          string
	Model       string
	Brand       string
	SupportABIs string
	Display     string
}

var devicePresetLibrary = []DevicePreset{
	{
		ID:          "xiaomi-11-cn",
		Model:       "M2011K2C",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "3200*1440",
	},
	{
		ID:          "xiaomi-11-pro-cn",
		Model:       "M2102K1AC",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "3200*1440",
	},
	{
		ID:          "xiaomi-k40-cn",
		Model:       "M2012K11AC",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "xiaomi-12-cn",
		Model:       "2201123C",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "xiaomi-12-pro-cn",
		Model:       "2201122C",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "3200*1440",
	},
	{
		ID:          "xiaomi-12s-cn",
		Model:       "2206123SC",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "xiaomi-12s-pro-cn",
		Model:       "2206122SC",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "3200*1440",
	},
	{
		ID:          "xiaomi-civi-1s-cn",
		Model:       "2203129SC",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "xiaomi-13-cn",
		Model:       "2211133C",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "xiaomi-13-pro-cn",
		Model:       "2210132C",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "3200*1440",
	},
	{
		ID:          "xiaomi-13-ultra-cn",
		Model:       "2304FPN6DC",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "3200*1440",
	},
	{
		ID:          "xiaomi-civi-3-cn",
		Model:       "23046PNC9C",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "xiaomi-14-cn",
		Model:       "23127PN0CC",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2670*1200",
	},
	{
		ID:          "xiaomi-14-pro-cn",
		Model:       "23116PN5BC",
		Brand:       "Xiaomi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "3200*1440",
	},
	{
		ID:          "redmi-k40-cn",
		Model:       "M2012K11C",
		Brand:       "Redmi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "redmi-k50-cn",
		Model:       "22021211RC",
		Brand:       "Redmi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "3200*1440",
	},
	{
		ID:          "redmi-k50-ultra-cn",
		Model:       "22081212C",
		Brand:       "Redmi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2712*1220",
	},
	{
		ID:          "redmi-k60-cn",
		Model:       "23013RK75C",
		Brand:       "Redmi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "3200*1440",
	},
	{
		ID:          "redmi-k60-pro-cn",
		Model:       "22127RK46C",
		Brand:       "Redmi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "3200*1440",
	},
	{
		ID:          "redmi-k70-cn",
		Model:       "2311DRK48C",
		Brand:       "Redmi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2712*1220",
	},
	{
		ID:          "redmi-k70-pro-cn",
		Model:       "23117RK66C",
		Brand:       "Redmi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "3200*1440",
	},
	{
		ID:          "redmi-note-11-pro-cn",
		Model:       "21091116C",
		Brand:       "Redmi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "redmi-note-12-pro-cn",
		Model:       "22101316C",
		Brand:       "Redmi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "redmi-note-12-turbo-cn",
		Model:       "23049RAD8C",
		Brand:       "Redmi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "redmi-note-13-pro-cn",
		Model:       "2312DRA50C",
		Brand:       "Redmi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2712*1220",
	},
	{
		ID:          "redmi-note-13-pro-plus-cn",
		Model:       "23090RA98C",
		Brand:       "Redmi",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2712*1220",
	},
	{
		ID:          "vivo-x60-cn",
		Model:       "V2055A",
		Brand:       "vivo",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2376*1080",
	},
	{
		ID:          "vivo-x80-cn",
		Model:       "V2183A",
		Brand:       "vivo",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "vivo-x90-cn",
		Model:       "V2241A",
		Brand:       "vivo",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2800*1260",
	},
	{
		ID:          "oppo-reno5-cn",
		Model:       "PEGM00",
		Brand:       "OPPO",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "oppo-find-x5-cn",
		Model:       "PFEM10",
		Brand:       "OPPO",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "oppo-reno8-cn",
		Model:       "PGAM10",
		Brand:       "OPPO",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "oppo-find-x6-cn",
		Model:       "PGFM10",
		Brand:       "OPPO",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2772*1240",
	},
	{
		ID:          "huawei-p40-cn",
		Model:       "ANA-AN00",
		Brand:       "HUAWEI",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2340*1080",
	},
	{
		ID:          "huawei-p50-cn",
		Model:       "ABR-AL00",
		Brand:       "HUAWEI",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2700*1224",
	},
	{
		ID:          "huawei-p60-cn",
		Model:       "MNA-AL00",
		Brand:       "HUAWEI",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2700*1220",
	},
	{
		ID:          "oneplus-ace-cn",
		Model:       "PGKM10",
		Brand:       "OnePlus",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2412*1080",
	},
	{
		ID:          "oneplus-11-cn",
		Model:       "PHB110",
		Brand:       "OnePlus",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "3216*1440",
	},
	{
		ID:          "oneplus-ace-2-cn",
		Model:       "PHK110",
		Brand:       "OnePlus",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2772*1240",
	},
	{
		ID:          "honor-70-cn",
		Model:       "FNE-AN00",
		Brand:       "HONOR",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2400*1080",
	},
	{
		ID:          "honor-90-cn",
		Model:       "REA-AN00",
		Brand:       "HONOR",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2664*1200",
	},
	{
		ID:          "realme-gt-neo3-cn",
		Model:       "RMX3560",
		Brand:       "realme",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2412*1080",
	},
	{
		ID:          "realme-gt5-cn",
		Model:       "RMX3820",
		Brand:       "realme",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2772*1240",
	},
	{
		ID:          "samsung-s23-cn",
		Model:       "SM-S9110",
		Brand:       "samsung",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2340*1080",
	},
	{
		ID:          "samsung-s24-cn",
		Model:       "SM-S9210",
		Brand:       "samsung",
		SupportABIs: "arm64-v8a,armeabi-v7a,armeabi",
		Display:     "2340*1080",
	},
}

func CompleteDeviceProfile(profile DeviceProfile) (DeviceProfile, error) {
	profile = normalizedDeviceProfile(profile)

	if needsDevicePreset(profile) {
		preset, err := selectDevicePreset(profile)
		if err != nil {
			return DeviceProfile{}, err
		}
		applyDevicePreset(&profile, preset)
	}

	var err error
	if strings.TrimSpace(profile.AndroidID) == "" {
		profile.AndroidID, err = randomString(deviceHexAlphabet, 16)
		if err != nil {
			return DeviceProfile{}, err
		}
	}
	if strings.TrimSpace(profile.MACAddress) == "" {
		profile.MACAddress, err = randomMACAddress()
		if err != nil {
			return DeviceProfile{}, err
		}
	}
	if strings.TrimSpace(profile.IMEI) == "" {
		profile.IMEI, err = randomIMEI()
		if err != nil {
			return DeviceProfile{}, err
		}
	}
	if strings.TrimSpace(profile.RuntimeUDID) == "" {
		profile.RuntimeUDID, err = randomString(deviceUpperAlphaNum, 36)
		if err != nil {
			return DeviceProfile{}, err
		}
	}
	if strings.TrimSpace(profile.UserProfileUDID) == "" {
		profile.UserProfileUDID, err = prefixedRandomString("XXA", deviceUpperAlphaNum, 34)
		if err != nil {
			return DeviceProfile{}, err
		}
	}
	if strings.TrimSpace(profile.CurBuvid) == "" {
		profile.CurBuvid, err = prefixedRandomString("XZA", deviceUpperAlphaNum, 34)
		if err != nil {
			return DeviceProfile{}, err
		}
	}

	return normalizedDeviceProfile(profile), nil
}

func normalizedDeviceProfile(profile DeviceProfile) DeviceProfile {
	normalizeDeviceProfile(&profile)
	return profile
}

func needsDevicePreset(profile DeviceProfile) bool {
	return strings.TrimSpace(profile.PresetID) == "" ||
		strings.TrimSpace(profile.Model) == "" ||
		strings.TrimSpace(profile.Brand) == "" ||
		strings.TrimSpace(profile.SupportABIs) == "" ||
		strings.TrimSpace(profile.Display) == ""
}

func selectDevicePreset(profile DeviceProfile) (DevicePreset, error) {
	if preset, ok := matchDevicePreset(profile); ok {
		return preset, nil
	}
	return randomDevicePreset()
}

func matchDevicePreset(profile DeviceProfile) (DevicePreset, bool) {
	presetID := strings.TrimSpace(strings.ToLower(profile.PresetID))
	if presetID != "" {
		for _, preset := range devicePresetLibrary {
			if strings.EqualFold(preset.ID, presetID) {
				return preset, true
			}
		}
	}

	model := strings.TrimSpace(profile.Model)
	brand := strings.TrimSpace(profile.Brand)
	supportABIs := strings.TrimSpace(profile.SupportABIs)
	display := strings.TrimSpace(profile.Display)
	if model == "" || brand == "" || supportABIs == "" || display == "" {
		return DevicePreset{}, false
	}

	for _, preset := range devicePresetLibrary {
		if strings.EqualFold(preset.Model, model) &&
			strings.EqualFold(preset.Brand, brand) &&
			strings.EqualFold(preset.SupportABIs, supportABIs) &&
			strings.EqualFold(preset.Display, display) {
			return preset, true
		}
	}
	return DevicePreset{}, false
}

func randomDevicePreset() (DevicePreset, error) {
	if len(devicePresetLibrary) == 0 {
		return DevicePreset{}, fmt.Errorf("device preset library is empty")
	}
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(devicePresetLibrary))))
	if err != nil {
		return DevicePreset{}, fmt.Errorf("select device preset: %w", err)
	}
	return devicePresetLibrary[index.Int64()], nil
}

func applyDevicePreset(profile *DeviceProfile, preset DevicePreset) {
	if profile == nil {
		return
	}
	profile.PresetID = preset.ID
	profile.Model = preset.Model
	profile.Brand = preset.Brand
	profile.SupportABIs = preset.SupportABIs
	profile.Display = preset.Display
}
