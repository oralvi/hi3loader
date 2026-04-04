package service

import (
	"strings"

	"hi3loader/internal/config"
	"hi3loader/internal/gameclient"
)

func (s *Service) autoPopulateRuntimePaths() {
	if s == nil {
		return
	}

	s.mu.Lock()
	if s.cfg == nil {
		s.mu.Unlock()
		return
	}
	nextCfg := s.cfg.Clone()
	s.mu.Unlock()

	changed := false
	gamePathNote := ""
	launcherPathNote := ""

	if strings.TrimSpace(nextCfg.GamePath) == "" {
		gamePath, _, err := gameclient.DetectGameInstallPathFromRegistry()
		if err != nil {
			s.logf("自动获取游戏路径失败，请手动添加")
			gamePathNote = "自动获取失败，请手动添加"
		} else {
			nextCfg.GamePath = gamePath
			changed = true
			gamePathNote = "已从注册表自动读取"
			s.logf("已从注册表自动获取游戏路径: %s", gamePath)
		}
	}

	if strings.TrimSpace(nextCfg.LauncherPath) == "" {
		launcherPath, _, err := gameclient.DetectLauncherExecutableFromRegistry()
		if err != nil {
			s.logf("自动获取启动器路径失败，请手动添加")
			launcherPathNote = "自动获取失败，请手动添加"
		} else {
			nextCfg.LauncherPath = launcherPath
			changed = true
			launcherPathNote = "已从注册表自动读取"
			s.logf("已从注册表自动获取启动器路径: %s", launcherPath)
		}
	}

	s.mu.Lock()
	s.gamePathNote = gamePathNote
	s.launcherPathNote = launcherPathNote
	s.mu.Unlock()

	if !changed {
		return
	}

	if err := config.Save(s.cfgPath, nextCfg); err != nil {
		s.logf("保存自动获取的运行路径失败: %v", err)
		return
	}

	s.mu.Lock()
	s.cfg = nextCfg
	s.mu.Unlock()
}
