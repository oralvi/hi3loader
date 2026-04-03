package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"hi3loader/internal/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx                        context.Context
	svc                        *service.Service
	updatePromptMu             sync.Mutex
	startupUpdatePromptPending bool
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
			a.maybePromptGameUpdateWhenReady(state)
		},
	})
}

func (a *App) shutdown(ctx context.Context) {
	_ = a.svc.Close(context.Background())
}

func (a *App) Bootstrap() (service.State, error) {
	state, err := a.svc.Bootstrap(context.Background())
	if err == nil {
		a.setStartupUpdatePromptPending(strings.TrimSpace(state.Config.GamePath) != "")
		a.maybePromptGameUpdateWhenReady(state)
	}
	return state, err
}

func (a *App) setStartupUpdatePromptPending(pending bool) {
	a.updatePromptMu.Lock()
	a.startupUpdatePromptPending = pending
	a.updatePromptMu.Unlock()
}

func (a *App) maybePromptGameUpdateWhenReady(state service.State) {
	if a == nil || a.svc == nil {
		return
	}
	if strings.TrimSpace(state.Config.GamePath) == "" || state.RuntimePreparing || !state.APIReady {
		return
	}

	a.updatePromptMu.Lock()
	if !a.startupUpdatePromptPending {
		a.updatePromptMu.Unlock()
		return
	}
	a.startupUpdatePromptPending = false
	a.updatePromptMu.Unlock()

	a.maybePromptGameUpdateOnStartup()
}

func (a *App) LogSnapshot() []service.LogEntry {
	return a.svc.LogSnapshot()
}

func (a *App) SaveFeatureSettings(gamePath string, autoClose, autoClip, panelBlur bool, opacity float64) (service.State, error) {
	previousGamePath := ""
	if cfg := a.svc.Config(); cfg != nil {
		previousGamePath = cfg.GamePath
	}

	state, err := a.svc.SaveFeatureSettings(strings.TrimSpace(gamePath), autoClose, autoClip, panelBlur, opacity)
	if err == nil && shouldPromptGameUpdateAfterSave(previousGamePath, state.Config.GamePath) {
		a.maybePromptGameUpdateAfterSave()
	}
	return state, err
}

func (a *App) UpdateBackground(backgroundPath string, opacity float64) (service.State, error) {
	return a.svc.UpdateBackground(strings.TrimSpace(backgroundPath), opacity)
}

func (a *App) Login(account, password string) (service.LoginResult, error) {
	return a.svc.Login(context.Background(), strings.TrimSpace(account), password, false)
}

func (a *App) SelectSavedAccount(account string) (service.State, error) {
	return a.svc.SelectSavedAccount(strings.TrimSpace(account))
}

func (a *App) ClearCurrentAccount() (service.State, error) {
	return a.svc.ClearCurrentAccount()
}

func (a *App) PauseMonitor() {
	a.svc.PauseMonitor()
}

func (a *App) ResumeMonitor() {
	a.svc.ResumeMonitor()
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
		Title:                "Select Honkai Impact 3 install directory",
		DefaultDirectory:     defaultDir,
		CanCreateDirectories: false,
		ShowHiddenFiles:      false,
	})
}

func (a *App) BrowseLauncherPath() (string, error) {
	cfg := a.svc.Config()
	defaultDir := ""
	if launcherPath := strings.TrimSpace(cfg.LauncherPath); launcherPath != "" {
		if _, err := os.Stat(launcherPath); err == nil {
			defaultDir = filepath.Dir(launcherPath)
		}
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Select official launcher executable",
		DefaultDirectory: defaultDir,
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Executable Files",
				Pattern:     "*.exe",
			},
		},
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
		Title:            "Select background image",
		DefaultDirectory: defaultDir,
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Image Files",
				Pattern:     "*.png;*.jpg;*.jpeg;*.webp",
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

func (a *App) ScanTicket(ticket string) (service.ScanResult, error) {
	return a.svc.ScanTicket(context.Background(), strings.TrimSpace(ticket))
}

func (a *App) ScanWindow() (service.ScanWindowResult, error) {
	return a.svc.ScanWindow(context.Background())
}

func (a *App) SaveCredentialSettings(asteriskName, loaderAPIBaseURL string) (service.State, error) {
	return a.svc.SaveCredentialSettings(strings.TrimSpace(asteriskName), strings.TrimSpace(loaderAPIBaseURL))
}

func (a *App) SaveLauncherPath(launcherPath string) (service.State, error) {
	return a.svc.SaveLauncherPath(strings.TrimSpace(launcherPath))
}

func (a *App) RecordClientMessage(message string) {
	a.svc.RecordClientMessage(message)
}

func (a *App) LoadLocaleMessages() map[string]map[string]string {
	return a.svc.LoadLocaleMessages()
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
