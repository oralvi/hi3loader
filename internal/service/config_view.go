package service

import (
	"strconv"
	"strings"

	"hi3loader/internal/config"
)

type ConfigView struct {
	Account            string             `json:"account"`
	SavedAccounts      []SavedAccountView `json:"saved_accounts,omitempty"`
	AutoClose          bool               `json:"auto_close"`
	GamePath           string             `json:"game_path,omitempty"`
	LauncherPath       string             `json:"launcher_path,omitempty"`
	UID                int64              `json:"uid"`
	LastLoginSucc      bool               `json:"last_login_succ"`
	AccountLogin       bool               `json:"account_login"`
	AsteriskName       string             `json:"asterisk_name,omitempty"`
	LoaderAPIBaseURL   string             `json:"loader_api_base_url,omitempty"`
	AutoWindowCapture  bool               `json:"auto_window_capture"`
	BackgroundOpacity  float64            `json:"background_opacity"`
	PanelBlur          bool               `json:"panel_blur"`
	RememberPassword   bool               `json:"remember_password"`
	HasPassword        bool               `json:"has_password"`
	HasAccessKey       bool               `json:"has_access_key"`
	HasBackgroundImage bool               `json:"has_background_image"`
	MaskedPassword     string             `json:"masked_password,omitempty"`
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
	active, _ := cfg.CurrentSavedAccount()

	view := ConfigView{
		Account:            active.Account,
		SavedAccounts:      buildSavedAccountViews(cfg.Accounts),
		AutoClose:          cfg.AutoClose,
		GamePath:           cfg.GamePath,
		LauncherPath:       cfg.LauncherPath,
		UID:                active.UID,
		LastLoginSucc:      active.LastLoginSucc,
		AccountLogin:       cfg.AccountLogin,
		AsteriskName:       cfg.AsteriskName,
		LoaderAPIBaseURL:   cfg.LoaderAPIBaseURL,
		AutoWindowCapture:  cfg.AutoWindowCapture,
		BackgroundOpacity:  cfg.BackgroundOpacity,
		PanelBlur:          cfg.PanelBlur,
		RememberPassword:   active.RememberPassword,
		HasPassword:        active.RememberPassword && strings.TrimSpace(active.Password) != "",
		HasAccessKey:       strings.TrimSpace(active.AccessKey) != "",
		HasBackgroundImage: strings.TrimSpace(cfg.BackgroundImage) != "",
	}

	if view.HasPassword {
		view.MaskedPassword = maskSecret(active.Password)
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
