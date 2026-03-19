package service

import (
	"strings"

	"hi3loader/internal/config"
)

type ConfigView struct {
	Account            string  `json:"account"`
	ClipCheck          bool    `json:"clip_check"`
	AutoClose          bool    `json:"auto_close"`
	GamePath           string  `json:"game_path,omitempty"`
	UID                int64   `json:"uid"`
	LastLoginSucc      bool    `json:"last_login_succ"`
	AccountLogin       bool    `json:"account_login"`
	BHVer              string  `json:"bh_ver"`
	BiliPkgVer         int     `json:"bili_pkg_ver,omitempty"`
	AutoClip           bool    `json:"auto_clip"`
	BackgroundOpacity  float64 `json:"background_opacity"`
	PanelBlur          bool    `json:"panel_blur"`
	HasPassword        bool    `json:"has_password"`
	HasAccessKey       bool    `json:"has_access_key"`
	HasHI3UID          bool    `json:"has_hi3uid"`
	HasBiliHitoken     bool    `json:"has_bilihitoken"`
	HasDispatchData    bool    `json:"has_dispatch_data"`
	HasBackgroundImage bool    `json:"has_background_image"`
	MaskedPassword     string  `json:"masked_password,omitempty"`
	MaskedHI3UID       string  `json:"masked_hi3uid,omitempty"`
	MaskedBiliHitoken  string  `json:"masked_bilihitoken,omitempty"`
}

func buildConfigView(cfg *config.Config) ConfigView {
	if cfg == nil {
		cfg = config.Default()
	}

	_, _, hasDispatch := cfg.DispatchSnapshot()

	view := ConfigView{
		Account:            cfg.Account,
		ClipCheck:          cfg.ClipCheck,
		AutoClose:          cfg.AutoClose,
		GamePath:           cfg.GamePath,
		UID:                cfg.UID,
		LastLoginSucc:      cfg.LastLoginSucc,
		AccountLogin:       cfg.AccountLogin,
		BHVer:              cfg.BHVer,
		BiliPkgVer:         cfg.BiliPkgVer,
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
