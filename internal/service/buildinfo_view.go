package service

import (
	"strings"
	"time"

	"hi3loader/internal/buildinfo"
)

type BuildInfoView struct {
	Version   string `json:"version,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
	Developer string `json:"developer,omitempty"`
	License   string `json:"license,omitempty"`
}

func currentBuildInfo() BuildInfoView {
	return BuildInfoView{
		Version:   strings.TrimSpace(buildinfo.AppVersion),
		BuildDate: formatBuildDate(buildinfo.EffectiveBuildStamp()),
		Developer: "Oralvi Sakura",
		License:   "MIT",
	}
}

func formatBuildDate(stamp string) string {
	stamp = strings.TrimSpace(stamp)
	if stamp == "" {
		return ""
	}
	if strings.HasPrefix(stamp, "r") && len(stamp) == 13 {
		if parsed, err := time.ParseInLocation("060102150405", stamp[1:], time.Local); err == nil {
			return parsed.Format("2006-01-02 15:04:05 -07:00")
		}
	}
	if strings.HasPrefix(stamp, "dev+") {
		if tail := stamp[strings.LastIndex(stamp, "+")+1:]; len(tail) == 12 {
			if parsed, err := time.ParseInLocation("060102150405", tail, time.Local); err == nil {
				return parsed.Format("2006-01-02 15:04:05 -07:00")
			}
		}
	}
	return stamp
}
