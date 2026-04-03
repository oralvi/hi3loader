package bridge

import (
	"net/url"
	"os"
	"strings"

	"hi3loader/internal/buildinfo"
)

const (
	defaultClientName    = "hi3loader-helper"
	defaultTransportMode = "local"
	defaultPlatform      = "windows-amd64"
)

func DefaultClientMeta() ClientMeta {
	return ClientMetaForBaseURL("")
}

func ClientMetaForBaseURL(baseURL string) ClientMeta {
	return ClientMeta{
		ClientName:       defaultClientName,
		ClientVersion:    strings.TrimSpace(buildinfo.AppVersion),
		BuildFingerprint: buildinfo.EffectiveBuildFingerprint(),
		Platform:         defaultPlatform,
		Locale:           detectLocale(),
		TransportMode:    transportModeForBaseURL(baseURL),
	}
}

func detectLocale() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := normalizeLocale(os.Getenv(key)); value != "" {
			return value
		}
	}
	return "und"
}

func normalizeLocale(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.IndexAny(value, ".@"); idx >= 0 {
		value = value[:idx]
	}
	return value
}

func transportModeForBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return defaultTransportMode
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return defaultTransportMode
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	switch host {
	case "", "127.0.0.1", "localhost", "::1":
		return "local"
	default:
		return "remote"
	}
}
