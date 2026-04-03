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
