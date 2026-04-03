package service

import (
	"context"
	"testing"

	"hi3loader/internal/bridge"
	"hi3loader/internal/config"
)

func TestDetectGameUpdateSkipsEmptyPath(t *testing.T) {
	oldRead := readInstalledGameVersion
	oldFetch := fetchRuntimeProfileForGame
	defer func() {
		readInstalledGameVersion = oldRead
		fetchRuntimeProfileForGame = oldFetch
	}()

	readCalled := false
	fetchCalled := false
	readInstalledGameVersion = func(string) (string, error) {
		readCalled = true
		return "", nil
	}
	fetchRuntimeProfileForGame = func(context.Context, string, bridge.ClientMeta) (bridge.RuntimeProfile, error) {
		fetchCalled = true
		return bridge.RuntimeProfile{}, nil
	}

	svc := &Service{cfg: config.Default()}
	status, err := svc.DetectGameUpdate(context.Background())
	if err != nil {
		t.Fatalf("detect game update: %v", err)
	}
	if readCalled || fetchCalled {
		t.Fatal("expected update detection to skip empty game path")
	}
	if status.Outdated {
		t.Fatal("expected empty game path to skip update prompt")
	}
}

func TestDetectGameUpdateReturnsOutdatedStatus(t *testing.T) {
	oldRead := readInstalledGameVersion
	oldFetch := fetchRuntimeProfileForGame
	defer func() {
		readInstalledGameVersion = oldRead
		fetchRuntimeProfileForGame = oldFetch
	}()

	readInstalledGameVersion = func(string) (string, error) {
		return "8.7.0", nil
	}
	fetchRuntimeProfileForGame = func(context.Context, string, bridge.ClientMeta) (bridge.RuntimeProfile, error) {
		return bridge.RuntimeProfile{GameVer: "8.8.0"}, nil
	}

	cfg := config.Default()
	cfg.GamePath = `E:\games\Honkai Impact 3rd Game`
	cfg.LoaderAPIBaseURL = "https://127.0.0.1:50259"
	svc := &Service{cfg: cfg}

	status, err := svc.DetectGameUpdate(context.Background())
	if err != nil {
		t.Fatalf("detect game update: %v", err)
	}
	if !status.Outdated {
		t.Fatal("expected local version to be marked outdated")
	}
	if status.LocalVersion != "8.7.0" || status.RemoteVersion != "8.8.0" {
		t.Fatalf("unexpected update status: %+v", status)
	}
}

func TestDetectGameUpdateSkipsWhenCurrent(t *testing.T) {
	oldRead := readInstalledGameVersion
	oldFetch := fetchRuntimeProfileForGame
	defer func() {
		readInstalledGameVersion = oldRead
		fetchRuntimeProfileForGame = oldFetch
	}()

	readInstalledGameVersion = func(string) (string, error) {
		return "8.8.0", nil
	}
	fetchRuntimeProfileForGame = func(context.Context, string, bridge.ClientMeta) (bridge.RuntimeProfile, error) {
		return bridge.RuntimeProfile{GameVer: "8.8.0"}, nil
	}

	cfg := config.Default()
	cfg.GamePath = `E:\games\Honkai Impact 3rd Game`
	cfg.LoaderAPIBaseURL = "https://127.0.0.1:50259"
	svc := &Service{cfg: cfg}

	status, err := svc.DetectGameUpdate(context.Background())
	if err != nil {
		t.Fatalf("detect game update: %v", err)
	}
	if status.Outdated {
		t.Fatalf("expected equal versions to skip update prompt: %+v", status)
	}
}
