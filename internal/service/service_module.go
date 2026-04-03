package service

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"hi3loader/internal/bridge"
)

const (
	moduleExecutableName = "loader-core.exe"
	moduleStartTimeout   = 30 * time.Second
	moduleProbeInterval  = 300 * time.Millisecond
	moduleLoopbackHost   = "127.0.0.1"
)

type moduleRuntime struct {
	mu       sync.Mutex
	baseDir  string
	exePath  string
	endpoint string
	cmd      *exec.Cmd
}

func newModuleRuntime() *moduleRuntime {
	baseDir := "."
	if exePath, err := os.Executable(); err == nil {
		baseDir = filepath.Dir(exePath)
	}
	return &moduleRuntime{
		baseDir: baseDir,
		exePath: filepath.Join(baseDir, moduleExecutableName),
	}
}

func (m *moduleRuntime) exists() bool {
	if m == nil {
		return false
	}
	info, err := os.Stat(m.exePath)
	return err == nil && !info.IsDir()
}

func (m *moduleRuntime) ensureRunning(ctx context.Context) (string, error) {
	if !m.exists() {
		return "", nil
	}

	if endpoint, ok := m.currentEndpoint(); ok && m.endpointReachable(ctx, endpoint) {
		return endpoint, nil
	}

	endpoint, err := m.allocateEndpoint()
	if err != nil {
		return "", err
	}
	if err := m.start(endpoint); err != nil {
		return "", err
	}
	return m.waitUntilReady(ctx, endpoint)
}

func (m *moduleRuntime) stop() {
	if m == nil {
		return
	}

	m.mu.Lock()
	cmd := m.cmd
	m.cmd = nil
	m.endpoint = ""
	m.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}

func (m *moduleRuntime) start(endpoint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		return nil
	}
	if err := m.stopExistingProcess(); err != nil {
		return err
	}

	cmd := exec.Command(m.exePath, "--background", "--addr", strings.TrimPrefix(endpoint, "https://"))
	cmd.Dir = m.baseDir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}

	if err := cmd.Start(); err != nil {
		if devNull != nil {
			_ = devNull.Close()
		}
		return fmt.Errorf("start %s: %w", moduleExecutableName, err)
	}

	m.endpoint = endpoint
	m.cmd = cmd
	go func(current *exec.Cmd, closer *os.File) {
		_ = current.Wait()
		if closer != nil {
			_ = closer.Close()
		}
		m.mu.Lock()
		if m.cmd == current {
			m.cmd = nil
			m.endpoint = ""
		}
		m.mu.Unlock()
	}(cmd, devNull)

	return nil
}

func (m *moduleRuntime) stopExistingProcess() error {
	target := strings.ReplaceAll(m.exePath, "'", "''")
	script := `$target = [System.IO.Path]::GetFullPath('` + target + `');
$name = [System.IO.Path]::GetFileName($target);
Get-CimInstance Win32_Process -Filter ("Name='" + $name.Replace("'", "''") + "'") |
  Where-Object {
    $_.ExecutablePath -and
    [string]::Equals(
      [System.IO.Path]::GetFullPath($_.ExecutablePath),
      $target,
      [System.StringComparison]::OrdinalIgnoreCase
    )
  } |
  ForEach-Object {
    Stop-Process -Id $_.ProcessId -Force -ErrorAction Stop
  }`
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		script,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stop existing %s: %w (%s)", moduleExecutableName, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (m *moduleRuntime) waitUntilReady(ctx context.Context, endpoint string) (string, error) {
	waitCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, moduleStartTimeout)
		defer cancel()
	}

	ticker := time.NewTicker(moduleProbeInterval)
	defer ticker.Stop()

	for {
		if m.endpointReachable(waitCtx, endpoint) {
			return endpoint, nil
		}

		select {
		case <-waitCtx.Done():
			return "", fmt.Errorf("timed out waiting for %s to become ready", moduleExecutableName)
		case <-ticker.C:
		}
	}
}

func (m *moduleRuntime) endpointReachable(parent context.Context, endpoint string) bool {
	probeCtx := parent
	if _, hasDeadline := parent.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		probeCtx, cancel = context.WithTimeout(parent, 2*time.Second)
		defer cancel()
	}

	if err := bridge.ProbeLoaderAPI(probeCtx, endpoint, bridge.ClientMetaForBaseURL(endpoint)); err != nil {
		return false
	}
	return true
}

func (m *moduleRuntime) currentEndpoint() (string, bool) {
	if m == nil {
		return "", false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	endpoint := strings.TrimSpace(m.endpoint)
	return endpoint, endpoint != ""
}

func (m *moduleRuntime) allocateEndpoint() (string, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(moduleLoopbackHost, "0"))
	if err != nil {
		return "", fmt.Errorf("reserve module port: %w", err)
	}
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return "", fmt.Errorf("resolve reserved module port")
	}
	port := addr.Port
	if err := listener.Close(); err != nil {
		return "", fmt.Errorf("release reserved module port: %w", err)
	}
	return fmt.Sprintf("https://%s:%d", moduleLoopbackHost, port), nil
}

func (s *Service) ensureBundledModule(ctx context.Context) (bool, error) {
	if s.module == nil || !s.ShouldUseBundledLoaderAPI() {
		return false, nil
	}
	if !s.module.exists() {
		s.Note("bundled module executable not found; continuing without local companion")
		return false, nil
	}
	if !bridge.HasEmbeddedServerAuthority() {
		s.Note("bundled module found without embedded authority; continuing without local companion")
		return false, nil
	}

	if endpoint, ok := s.module.currentEndpoint(); ok && s.module.endpointReachable(ctx, endpoint) {
		if err := s.AdoptBundledLoaderAPI(endpoint); err != nil {
			return false, err
		}
		return false, nil
	}
	if s.isRuntimePreparing() {
		return true, nil
	}

	s.setRuntimePreparing(true)
	s.logf("starting bundled runtime in background")
	go s.bootstrapBundledModule(context.Background())
	return true, nil
}

func (s *Service) bootstrapBundledModule(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, moduleStartTimeout)
	defer cancel()

	endpoint, err := s.module.ensureRunning(ctx)
	if err != nil {
		s.setRuntimePreparing(false)
		s.setError(fmt.Errorf("start bundled runtime: %w", err))
		return
	}

	if err := s.AdoptBundledLoaderAPI(endpoint); err != nil {
		s.setRuntimePreparing(false)
		s.setError(err)
		return
	}

	if err := s.refreshAPIHealth(context.Background()); err != nil {
		s.logf("loader api probe failed: %v", err)
	}
	s.setRuntimePreparing(false)
}

func isLoopbackLoaderAPI(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return host == "127.0.0.1" || host == "localhost"
}
