package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"hi3loader/internal/gameclient"
	"hi3loader/internal/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) maybePromptGameUpdateOnStartup() {
	go a.promptGameUpdate("startup")
}

func (a *App) maybePromptGameUpdateAfterSave() {
	go a.promptGameUpdate("path_saved")
}

func (a *App) promptGameUpdate(reason string) {
	if a == nil || a.ctx == nil || a.svc == nil {
		return
	}

	a.updatePromptMu.Lock()
	defer a.updatePromptMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	status, err := a.svc.DetectGameUpdate(ctx)
	if err != nil {
		a.svc.Note(fmt.Sprintf("skip game update prompt (%s): %v", reason, err))
		return
	}
	if !status.Outdated || strings.TrimSpace(status.LocalVersion) == "" || strings.TrimSpace(status.RemoteVersion) == "" {
		return
	}

	cfg := a.svc.Config()
	launcherPath := strings.TrimSpace(cfg.LauncherPath)
	if launcherPath == "" {
		a.showManualUpdateDialog(status)
		return
	}

	selection, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "检测到游戏更新",
		Message:       fmt.Sprintf("当前本地版本为 %s，远端最新版本为 %s。\n\n是否现在唤起官方启动器更新？", status.LocalVersion, status.RemoteVersion),
		DefaultButton: "Yes",
		CancelButton:  "No",
	})
	if err != nil {
		a.svc.Note(fmt.Sprintf("show game update prompt failed (%s): %v", reason, err))
		return
	}

	if strings.EqualFold(strings.TrimSpace(selection), "yes") {
		launchedPath, launchErr := gameclient.LaunchLauncher(launcherPath)
		if launchErr == nil {
			a.svc.Note(fmt.Sprintf("requested launcher update via %s (%s -> %s)", launchedPath, status.LocalVersion, status.RemoteVersion))
			return
		}
		a.svc.Note(fmt.Sprintf("launcher update handoff failed: %v", launchErr))
	}

	a.showManualUpdateDialog(status)
}

func (a *App) showManualUpdateDialog(status service.GameUpdateStatus) {
	_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.InfoDialog,
		Title:   "请稍后手动更新",
		Message: manualUpdateMessage(status),
	})
}

func manualUpdateMessage(status service.GameUpdateStatus) string {
	lines := []string{
		"请稍后手动打开官方启动器并完成游戏更新。",
	}
	if strings.TrimSpace(status.LocalVersion) != "" {
		lines = append(lines, fmt.Sprintf("当前本地版本：%s", status.LocalVersion))
	}
	if strings.TrimSpace(status.RemoteVersion) != "" {
		lines = append(lines, fmt.Sprintf("远端最新版本：%s", status.RemoteVersion))
	}
	return strings.Join(lines, "\n")
}

func shouldPromptGameUpdateAfterSave(previousPath, nextPath string) bool {
	previousPath = normalizeComparablePath(previousPath)
	nextPath = normalizeComparablePath(nextPath)
	return nextPath != "" && previousPath != nextPath
}

func normalizeComparablePath(path string) string {
	path = gameclient.NormalizePath(path)
	if path == "" {
		return ""
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return strings.ToLower(path)
	}
	return strings.ToLower(absPath)
}
