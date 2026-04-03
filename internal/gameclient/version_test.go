package gameclient

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadVersionFromConfigINI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.ini")
	content := "[General]\ngame_version=8.8.0\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config.ini: %v", err)
	}

	version, err := ReadVersion(dir)
	if err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version != "8.8.0" {
		t.Fatalf("unexpected version %q", version)
	}
}

func TestReadVersionRejectsMissingConfigVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.ini"), []byte("[General]\n"), 0o600); err != nil {
		t.Fatalf("write config.ini: %v", err)
	}
	if _, err := ReadVersion(dir); err == nil {
		t.Fatal("expected missing game_version in config.ini to fail")
	}
}

func TestCompareVersion(t *testing.T) {
	tests := []struct {
		name   string
		local  string
		remote string
		want   int
	}{
		{name: "older", local: "8.7.0", remote: "8.8.0", want: -1},
		{name: "same", local: "8.8.0", remote: "8.8.0", want: 0},
		{name: "newer", local: "8.8.1", remote: "8.8.0", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareVersion(tt.local, tt.remote)
			if err != nil {
				t.Fatalf("compare version: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected compare result %d, want %d", got, tt.want)
			}
		})
	}
}
