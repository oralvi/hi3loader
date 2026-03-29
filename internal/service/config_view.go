package service

import (
	"strconv"
	"strings"

	"hi3loader/internal/config"
)

type ConfigView struct {
	Account            string             `json:"account"`
	SavedAccounts      []SavedAccountView `json:"saved_accounts,omitempty"`
	ClipCheck          bool               `json:"clip_check"`
	AutoClose          bool               `json:"auto_close"`
	GamePath           string             `json:"game_path,omitempty"`
	UID                int64              `json:"uid"`
	LastLoginSucc      bool               `json:"last_login_succ"`
	AccountLogin       bool               `json:"account_login"`
	BHVer              string             `json:"bh_ver"`
	BiliPkgVer         int                `json:"bili_pkg_ver,omitempty"`
	AsteriskName       string             `json:"asterisk_name,omitempty"`
	AutoClip           bool               `json:"auto_clip"`
	BackgroundOpacity  float64            `json:"background_opacity"`
	PanelBlur          bool               `json:"panel_blur"`
	HasPassword        bool               `json:"has_password"`
	HasAccessKey       bool               `json:"has_access_key"`
	HasHI3UID          bool               `json:"has_hi3uid"`
	HasBiliHitoken     bool               `json:"has_bilihitoken"`
	HasDispatchData    bool               `json:"has_dispatch_data"`
	HasBackgroundImage bool               `json:"has_background_image"`
	MaskedPassword     string             `json:"masked_password,omitempty"`
	MaskedHI3UID       string             `json:"masked_hi3uid,omitempty"`
	MaskedBiliHitoken  string             `json:"masked_bilihitoken,omitempty"`
}

type SavedAccountView struct {
	Account     string `json:"account"`
	UName       string `json:"uname,omitempty"`
	UID         int64  `json:"uid,omitempty"`
	DisplayName string `json:"display_name"`
}

func buildConfigView(cfg *config.Config) ConfigView {
	if cfg == nil {
		cfg = config.Default()
	}

	_, _, hasDispatch := cfg.DispatchSnapshot()

	view := ConfigView{
		Account:            cfg.Account,
		SavedAccounts:      buildSavedAccountViews(cfg.Accounts),
		ClipCheck:          cfg.ClipCheck,
		AutoClose:          cfg.AutoClose,
		GamePath:           cfg.GamePath,
		UID:                cfg.UID,
		LastLoginSucc:      cfg.LastLoginSucc,
		AccountLogin:       cfg.AccountLogin,
		BHVer:              cfg.BHVer,
		BiliPkgVer:         cfg.BiliPkgVer,
		AsteriskName:       cfg.AsteriskName,
		AutoClip:           cfg.AutoClip,
		BackgroundOpacity:  cfg.BackgroundOpacity,
		PanelBlur:          cfg.PanelBlur,
		HasPassword:        strings.TrimSpace(cfg.Password) != "",
		HasAccessKey:       strings.TrimSpace(cfg.AccessKey) != "",
		HasHI3UID:          strings.TrimSpace(cfg.HI3UID) != "",
		HasBiliHitoken:     strings.TrimSpace(cfg.BILIHITOKEN) != "",
		HasDispatchData:    hasDispatch,
		HasBackgroundImage: strings.TrimSpace(cfg.BackgroundImage) != "",
	}

	if view.HasPassword {
		view.MaskedPassword = maskSecret(cfg.Password)
	}
	if view.HasHI3UID {
		view.MaskedHI3UID = maskSecret(cfg.HI3UID)
	}
	if view.HasBiliHitoken {
		view.MaskedBiliHitoken = maskSecret(cfg.BILIHITOKEN)
	}
	return view
}

func buildSavedAccountViews(accounts []config.SavedAccount) []SavedAccountView {
	if len(accounts) == 0 {
		return nil
	}

	unameCounts := make(map[string]int, len(accounts))
	for _, entry := range accounts {
		uname := strings.TrimSpace(entry.UName)
		if uname != "" {
			unameCounts[strings.ToLower(uname)]++
		}
	}

	views := make([]SavedAccountView, 0, len(accounts))
	for _, entry := range accounts {
		displayName := strings.TrimSpace(entry.UName)
		if displayName == "" {
			displayName = entry.Account
		} else if unameCounts[strings.ToLower(displayName)] > 1 && entry.UID != 0 {
			displayName = displayName + " · " + uidTail(entry.UID)
		}
		views = append(views, SavedAccountView{
			Account:     entry.Account,
			UName:       entry.UName,
			UID:         entry.UID,
			DisplayName: displayName,
		})
	}
	return views
}

func uidTail(uid int64) string {
	if uid == 0 {
		return "0000"
	}
	text := strconv.FormatInt(uid, 10)
	if len(text) <= 4 {
		return text
	}
	return text[len(text)-4:]
}
