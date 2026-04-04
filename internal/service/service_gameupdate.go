package service

import (
	"context"
	"strings"

	"hi3loader/internal/bridge"
	"hi3loader/internal/gameclient"
)

type GameUpdateStatus struct {
	LocalVersion  string
	RemoteVersion string
	Outdated      bool
}

type GameUpdatePrompt struct {
	LocalVersion      string `json:"localVersion"`
	RemoteVersion     string `json:"remoteVersion"`
	Outdated          bool   `json:"outdated"`
	LauncherAvailable bool   `json:"launcherAvailable"`
}

var (
	readInstalledGameVersion   = gameclient.ReadVersion
	fetchRuntimeProfileForGame = bridge.FetchRuntimeProfile
)

func (s *Service) DetectGameUpdate(ctx context.Context) (GameUpdateStatus, error) {
	cfg := s.Config()
	gamePath := strings.TrimSpace(cfg.GamePath)
	if gamePath == "" {
		return GameUpdateStatus{}, nil
	}

	localVersion, err := readInstalledGameVersion(gamePath)
	if err != nil {
		return GameUpdateStatus{}, err
	}

	baseURL := strings.TrimSpace(cfg.LoaderAPIBaseURL)
	if baseURL == "" {
		return GameUpdateStatus{LocalVersion: localVersion}, nil
	}

	s.beginAPIInteraction()
	profile, err := fetchRuntimeProfileForGame(ctx, baseURL, bridge.ClientMetaForBaseURL(baseURL))
	s.endAPIInteraction()
	if err != nil {
		s.setAPIReady(false)
		return GameUpdateStatus{LocalVersion: localVersion}, err
	}
	s.setAPIReady(true)

	remoteVersion := strings.TrimSpace(profile.GameVer)
	if remoteVersion == "" {
		return GameUpdateStatus{LocalVersion: localVersion}, nil
	}

	outdated, err := gameclient.IsOutdated(localVersion, remoteVersion)
	if err != nil {
		return GameUpdateStatus{}, err
	}
	return GameUpdateStatus{
		LocalVersion:  localVersion,
		RemoteVersion: remoteVersion,
		Outdated:      outdated,
	}, nil
}

func (s *Service) CheckGameUpdate(ctx context.Context) (GameUpdatePrompt, error) {
	status, err := s.DetectGameUpdate(ctx)
	if err != nil {
		return GameUpdatePrompt{}, err
	}

	cfg := s.Config()
	return GameUpdatePrompt{
		LocalVersion:      status.LocalVersion,
		RemoteVersion:     status.RemoteVersion,
		Outdated:          status.Outdated,
		LauncherAvailable: strings.TrimSpace(cfg.LauncherPath) != "",
	}, nil
}

func (s *Service) LaunchUpdater() error {
	cfg := s.Config()
	launchedPath, err := gameclient.LaunchLauncher(cfg.LauncherPath)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.lastAction = "launch_updater"
	s.lastError = ""
	s.lastErrorMessage = MessageRef{}
	s.mu.Unlock()

	s.logf("official launcher started via %s", launchedPath)
	s.emitState()
	return nil
}
