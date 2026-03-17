package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"hi3loader/internal/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context
	svc *service.Service
}

func NewApp(svc *service.Service) *App {
	return &App{svc: svc}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.svc.SetHooks(service.Hooks{
		OnLog: func(entry service.LogEntry) {
			runtime.EventsEmit(ctx, "log", entry)
		},
		OnState: func(state service.State) {
			runtime.EventsEmit(ctx, "state", state)
			if state.QuitRequested {
				runtime.EventsEmit(ctx, "quit-requested", state)
			}
		},
	})
}

func (a *App) shutdown(ctx context.Context) {
	_ = a.svc.Close(context.Background())
}

func (a *App) Bootstrap() (service.State, error) {
	return a.svc.Bootstrap(context.Background())
}

func (a *App) State() service.State {
	return a.svc.State()
}

func (a *App) UpdateConfig(gamePath string, clipCheck, autoClose, autoClip, panelBlur bool) (service.State, error) {
	return a.svc.UpdateConfig(strings.TrimSpace(gamePath), clipCheck, autoClose, autoClip, panelBlur)
}

func (a *App) UpdateBackground(backgroundPath string, opacity float64) (service.State, error) {
	return a.svc.UpdateBackground(strings.TrimSpace(backgroundPath), opacity)
}

func (a *App) Login(account, password string) (service.LoginResult, error) {
	return a.svc.Login(context.Background(), strings.TrimSpace(account), password, false)
}

func (a *App) LaunchGame() error {
	return a.svc.LaunchGame()
}

func (a *App) BrowseGamePath() (string, error) {
	cfg := a.svc.Config()
	defaultDir := strings.TrimSpace(cfg.GamePath)
	if defaultDir != "" {
		if _, err := os.Stat(defaultDir); err != nil {
			defaultDir = ""
		}
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "选择崩坏3游戏目录",
		DefaultDirectory:     defaultDir,
		CanCreateDirectories: false,
		ShowHiddenFiles:      false,
	})
}

func (a *App) BrowseBackgroundImage() (string, error) {
	cfg := a.svc.Config()
	defaultDir := ""
	if bgPath := strings.TrimSpace(cfg.BackgroundImage); bgPath != "" {
		if _, err := os.Stat(bgPath); err == nil {
			defaultDir = filepath.Dir(bgPath)
		}
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "选择背景图片",
		DefaultDirectory: defaultDir,
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Image Files",
				Pattern:     "*.png;*.jpg;*.jpeg;*.webp;*.bmp",
			},
		},
	})
}

func (a *App) BackgroundDataURL() string {
	return a.svc.BackgroundDataURL()
}

func (a *App) ResetBackground() (service.State, error) {
	return a.svc.ResetBackground()
}

func (a *App) OpenCaptcha() error {
	return a.svc.OpenCaptchaURL()
}

func (a *App) ScanTicket(ticket string) (map[string]any, error) {
	return a.svc.ScanTicket(context.Background(), strings.TrimSpace(ticket))
}

func (a *App) ScanURL(rawURL string) (map[string]any, error) {
	return a.svc.ScanURL(context.Background(), strings.TrimSpace(rawURL))
}

func (a *App) ScanClipboard() (bool, error) {
	return a.svc.ScanClipboardOnce(context.Background())
}

func (a *App) ScanWindow() (bool, error) {
	return a.svc.ScanWindowOnce(context.Background())
}

func (a *App) ManualRefreshDispatch(hi3uid, biliHitoken string) (service.State, error) {
	// Save credentials and attempt a dispatch refresh immediately
	if strings.TrimSpace(hi3uid) == "" || strings.TrimSpace(biliHitoken) == "" {
		return a.svc.State(), nil
	}
	return a.svc.ManualRefreshDispatch(context.Background(), strings.TrimSpace(hi3uid), strings.TrimSpace(biliHitoken))
}

func (a *App) ManualFetchBiliHitoken() (service.State, error) {
	return a.svc.ManualFetchBiliHitoken(context.Background())
}

func (a *App) SaveSetting(key string, value any) (service.State, error) {
	return a.svc.SaveSetting(key, value)
}

func (a *App) ResetQuitFlag() service.State {
	return a.svc.ResetQuitFlag()
}

func (a *App) RevealWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.Show(a.ctx)
	runtime.WindowShow(a.ctx)
}
