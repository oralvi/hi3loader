package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigPathPointsToExecutableDirectory(t *testing.T) {
	path := configPath()
	if filepath.Base(path) != "config.json" {
		t.Fatalf("unexpected config file name: %q", path)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute config path, got %q", path)
	}
	if strings.TrimSpace(filepath.Dir(path)) == "" {
		t.Fatalf("expected config path to include executable directory, got %q", path)
	}
}
