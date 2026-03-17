package service

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"hi3loader/internal/bilihitoken"
	"hi3loader/internal/bsgamesdk"
	"hi3loader/internal/captcha"
	"hi3loader/internal/config"
	"hi3loader/internal/debuglog"
	"hi3loader/internal/gameclient"
	"hi3loader/internal/mihoyosdk"
	"hi3loader/internal/netutil"
	"hi3loader/internal/qr"
	"hi3loader/internal/winwindow"

	"github.com/pkg/browser"
	"golang.design/x/clipboard"
)

const (
	defaultConfigPath         = "config.json"
	maxLogEntries             = 300
	ticketTTL                 = 30 * time.Second
	defaultSleepTime          = 3
	blockedSleepTime          = 8
	maxDispatchCaches         = 12
	managedBackgroundBaseName = "custom_background"
)

var (
	targetWindowPattern = regexp.MustCompile(`\x{5D29}\x{574F}3`)
	targetProcessNames  = []string{"bh3.exe"}
	sensitiveLogRules   = []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		{regexp.MustCompile(`(?i)("password"\s*:\s*")([^"]*)(")`), `${1}***${3}`},
		{regexp.MustCompile(`(?i)("access_key"\s*:\s*")([^"]*)(")`), `${1}***${3}`},
		{regexp.MustCompile(`(?i)("combo_token"\s*:\s*")([^"]*)(")`), `${1}***${3}`},
		{regexp.MustCompile(`(?i)("accountToken"\s*:\s*")([^"]*)(")`), `${1}***${3}`},
		{regexp.MustCompile(`(?i)(password=)([^&\s]+)`), `${1}***`},
		{regexp.MustCompile(`(?i)(access_key=)([^&\s]+)`), `${1}***`},
		{regexp.MustCompile(`(?i)(combo_token=)([^&\s]+)`), `${1}***`},
	}
)

type LogEntry struct {
	At      string `json:"at"`
	Message string `json:"message"`
}

type State struct {
	Config         config.Config `json:"config"`
	Running        bool          `json:"running"`
	ServerAddress  string        `json:"serverAddress"`
	ServerReady    bool          `json:"serverReady"`
	DispatchSource string        `json:"dispatchSource"`
	GamePathValid  bool          `json:"gamePathValid"`
	GamePathPrompt string        `json:"gamePathPrompt"`
	LogPath        string        `json:"logPath"`
	CaptchaURL     string        `json:"captchaURL"`
	CaptchaPending bool          `json:"captchaPending"`
	LastAction     string        `json:"lastAction"`
	LastError      string        `json:"lastError"`
	LastTicket     string        `json:"lastTicket"`
	LastQRCodeURL  string        `json:"lastQRCodeURL"`
	QuitRequested  bool          `json:"quitRequested"`
	Logs           []LogEntry    `json:"logs"`
}

type LoginResult struct {
	OK               bool           `json:"ok"`
	NeedsCaptcha     bool           `json:"needsCaptcha"`
	CaptchaURL       string         `json:"captchaURL,omitempty"`
	Message          string         `json:"message,omitempty"`
	UName            string         `json:"uname,omitempty"`
	BiliResponse     map[string]any `json:"biliResponse,omitempty"`
	UserInfo         map[string]any `json:"userInfo,omitempty"`
	VerifyResponse   map[string]any `json:"verifyResponse,omitempty"`
	DispatchResponse map[string]any `json:"dispatchResponse,omitempty"`
}

type Hooks struct {
	OnLog   func(LogEntry)
	OnState func(State)
}

type Service struct {
	cfgPath string

	bili   *bsgamesdk.Client
	mihoyo *mihoyosdk.Client

	mu                sync.RWMutex
	cfg               *config.Config
	server            *captcha.Server
	serverReady       bool
	serverStarted     bool
	running           bool
	loopCancel        context.CancelFunc
	captchaURL        string
	captchaPending    bool
	dispatchSource    string
	lastAction        string
	lastError         string
	lastTicket        string
	lastQRCodeURL     string
	quitRequested     bool
	logs              []LogEntry
	bhInfo            map[string]any
	backgroundDataURL string
	recentTickets     map[string]time.Time
	clipboardHash     string
	clipboardReady    bool
	clipboardErr      error
	hooks             Hooks
	fileLog           *debuglog.Logger
	loginMu           sync.Mutex
}

func New(cfgPath string) (*Service, error) {
	if cfgPath == "" {
		cfgPath = defaultConfigPath
	}

	cfg, err := config.LoadOrCreate(cfgPath)
	if err != nil {
		return nil, err
	}

	fileLog, _ := debuglog.New("logs", "hi3loader-debug.log")

	s := &Service{
		cfgPath:       cfgPath,
		cfg:           cfg,
		bili:          bsgamesdk.NewClient(),
		mihoyo:        mihoyosdk.NewClient(),
		fileLog:       fileLog,
		recentTickets: map[string]time.Time{},
	}
	s.server = captcha.NewServer("127.0.0.1:0", s.handleCaptchaResult)
	if dataURL, err := loadBackgroundDataURL(cfg.BackgroundImage); err == nil {
		s.backgroundDataURL = dataURL
	}
	return s, nil
}

func (s *Service) SetHooks(h Hooks) {
	s.mu.Lock()
	s.hooks = h
	s.mu.Unlock()
}

func (s *Service) Bootstrap(ctx context.Context) (State, error) {
	if err := s.Start(); err != nil {
		return State{}, err
	}

	_, _, _ = s.syncGameVersion()
	s.logAvailableDispatchSource()

	return s.State(), nil
}

func (s *Service) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.running = true
	s.loopCancel = cancel
	s.lastAction = "monitoring"
	s.mu.Unlock()

	s.initClipboard()
	go s.monitorLoop(ctx)
	s.emitState()
	return nil
}

func (s *Service) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	cancel := s.loopCancel
	s.loopCancel = nil
	s.running = false
	s.lastAction = "stopped"
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.emitState()
}

func (s *Service) pauseMonitorAfterSuccess() {
	s.mu.Lock()
	if !s.running {
		s.lastAction = "scan_complete"
		s.mu.Unlock()
		s.emitState()
		return
	}
	cancel := s.loopCancel
	s.loopCancel = nil
	s.running = false
	s.lastAction = "scan_complete"
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.emitState()
}

func (s *Service) Close(ctx context.Context) error {
	s.Stop()
	var err error
	if s.server != nil {
		err = s.server.Shutdown(ctx)
	}
	if s.fileLog != nil {
		s.fileLog.Writef("service shutdown")
		_ = s.fileLog.Close()
	}
	return err
}

func (s *Service) State() State {
	s.mu.RLock()
	cfg := s.cfg.Clone()
	running := s.running
	serverReady := s.serverReady
	dispatchSource := s.dispatchSource
	logPath := s.logPath()
	captchaURL := s.captchaURL
	captchaPending := s.captchaPending
	lastAction := s.lastAction
	lastError := s.lastError
	lastTicket := s.lastTicket
	lastQRCodeURL := s.lastQRCodeURL
	quitRequested := s.quitRequested
	logs := append([]LogEntry(nil), s.logs...)
	s.mu.RUnlock()

	gamePathValid, gamePathPrompt := evaluateGamePath(cfg.GamePath)

	return State{
		Config:         *cfg,
		Running:        running,
		ServerAddress:  s.server.Addr(),
		ServerReady:    serverReady,
		DispatchSource: dispatchSource,
		GamePathValid:  gamePathValid,
		GamePathPrompt: gamePathPrompt,
		LogPath:        logPath,
		CaptchaURL:     captchaURL,
		CaptchaPending: captchaPending,
		LastAction:     lastAction,
		LastError:      lastError,
		LastTicket:     lastTicket,
		LastQRCodeURL:  lastQRCodeURL,
		QuitRequested:  quitRequested,
		Logs:           logs,
	}
}

func (s *Service) Config() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Clone()
}

func (s *Service) UpdateConfig(gamePath string, clipCheck, autoClose, autoClip, panelBlur bool) (State, error) {
	resolvedGamePath := ""
	if strings.TrimSpace(gamePath) != "" {
		dir, err := gameclient.ResolveDir(gamePath)
		if err != nil {
			return State{}, err
		}
		resolvedGamePath = dir
	}

	s.mu.Lock()
	s.cfg.GamePath = resolvedGamePath
	s.cfg.ClipCheck = clipCheck
	s.cfg.AutoClose = autoClose
	s.cfg.AutoClip = autoClip
	s.cfg.PanelBlur = panelBlur
	s.cfg.SleepTime = defaultSleepTime
	err := config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	if err != nil {
		return State{}, err
	}

	if _, _, err := s.syncGameVersion(); err != nil {
		return State{}, err
	}
	_, _ = s.syncVersionAndDispatch(context.Background(), false)
	s.emitState()
	return s.State(), nil
}

// SaveSetting updates a single config field and persists the config.
func (s *Service) SaveSetting(key string, value any) (State, error) {
	// Temporary incoming-audit: write the raw incoming key/value so we can
	// trace whether frontend actually sent the HI3UID. This is short-lived
	// debugging instrumentation and can be removed after investigation.
	if s.fileLog != nil {
		s.fileLog.Writef("incoming SaveSetting: %s -> %v", key, value)
	}
	// Also append a compact text line to logs/save_incoming.log to ensure
	// we capture the event regardless of logger working directory.
	func() {
		f, err := os.OpenFile("logs/save_incoming.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()
		_, _ = f.WriteString(time.Now().Format(time.RFC3339Nano) + " " + key + " -> " + fmt.Sprint(value) + "\n")
	}()
	// capture old value for audit
	var oldVal string
	s.mu.Lock()
	if s.cfg != nil {
		// read current value into oldVal (best-effort)
		switch key {
		case "account":
			oldVal = s.cfg.Account
		case "password":
			oldVal = "<redacted>"
		case "HI3UID":
			oldVal = s.cfg.HI3UID
		case "BILIHITOKEN":
			oldVal = s.cfg.BILIHITOKEN
		case "panel_blur":
			oldVal = fmt.Sprintf("%v", s.cfg.PanelBlur)
		case "clip_check":
			oldVal = fmt.Sprintf("%v", s.cfg.ClipCheck)
		case "auto_clip":
			oldVal = fmt.Sprintf("%v", s.cfg.AutoClip)
		case "auto_close":
			oldVal = fmt.Sprintf("%v", s.cfg.AutoClose)
		case "background_opacity":
			oldVal = fmt.Sprintf("%v", s.cfg.BackgroundOpacity)
		case "game_path":
			oldVal = s.cfg.GamePath
		default:
			oldVal = "<unknown>"
		}
	}
	// modify in-memory config under lock
	switch key {
	case "account":
		s.cfg.Account = strings.TrimSpace(fmt.Sprintf("%v", value))
	case "password":
		s.cfg.Password = fmt.Sprintf("%v", value)
	case "HI3UID":
		s.cfg.HI3UID = strings.TrimSpace(fmt.Sprintf("%v", value))
	case "BILIHITOKEN":
		s.cfg.BILIHITOKEN = strings.TrimSpace(fmt.Sprintf("%v", value))
	case "panel_blur":
		if b, ok := value.(bool); ok {
			s.cfg.PanelBlur = b
		}
	case "clip_check":
		if b, ok := value.(bool); ok {
			s.cfg.ClipCheck = b
		}
	case "auto_clip":
		if b, ok := value.(bool); ok {
			s.cfg.AutoClip = b
		}
	case "auto_close":
		if b, ok := value.(bool); ok {
			s.cfg.AutoClose = b
		}
	case "background_opacity":
		// expect number (float64)
		if f, ok := value.(float64); ok {
			s.cfg.BackgroundOpacity = f
		}
	case "game_path":
		s.cfg.GamePath = strings.TrimSpace(fmt.Sprintf("%v", value))
	default:
		s.mu.Unlock()
		return s.State(), fmt.Errorf("unknown setting: %s", key)
	}
	// persist while still holding the lock to avoid races
	err := config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	// Audit log: record setting change (avoid logging sensitive password)
	if s.fileLog != nil {
		newVal := ""
		switch key {
		case "password":
			newVal = "<redacted>"
		case "HI3UID", "BILIHITOKEN", "account", "game_path":
			newVal = fmt.Sprintf("%v", value)
		case "panel_blur", "clip_check", "auto_clip", "auto_close":
			newVal = fmt.Sprintf("%v", value)
		case "background_opacity":
			newVal = fmt.Sprintf("%v", value)
		default:
			newVal = fmt.Sprintf("%v", value)
		}
		// redact long tokens for safety in logs (keep start/end)
		if key == "BILIHITOKEN" && len(newVal) > 8 {
			newVal = newVal[:4] + "..." + newVal[len(newVal)-4:]
		}
		if oldVal == "<redacted>" {
			s.fileLog.Writef("save setting: %s -> %s", key, newVal)
		} else {
			s.fileLog.Writef("save setting: %s '%s' -> '%s'", key, oldVal, newVal)
		}
	}
	if err != nil {
		return s.State(), err
	}

	// If HI3UID/BILIHITOKEN changed, do not auto-refresh dispatch here; leave it to manual action
	s.emitState()
	return s.State(), nil
}

func (s *Service) UpdateBackground(backgroundPath string, opacity float64) (State, error) {
	opacity = clampBackgroundOpacity(opacity)
	backgroundPath = strings.TrimSpace(backgroundPath)

	s.mu.RLock()
	currentPath := s.cfg.BackgroundImage
	currentDataURL := s.backgroundDataURL
	s.mu.RUnlock()

	targetPath := currentPath
	targetDataURL := currentDataURL

	switch {
	case backgroundPath == "":
		targetPath = currentPath
	case samePath(backgroundPath, currentPath):
		if targetDataURL == "" && strings.TrimSpace(targetPath) != "" {
			dataURL, err := loadBackgroundDataURL(targetPath)
			if err != nil {
				return State{}, err
			}
			targetDataURL = dataURL
		}
	default:
		destPath, dataURL, err := s.copyManagedBackground(backgroundPath, currentPath)
		if err != nil {
			return State{}, err
		}
		targetPath = destPath
		targetDataURL = dataURL
	}

	s.mu.Lock()
	s.cfg.BackgroundImage = targetPath
	s.cfg.BackgroundOpacity = opacity
	s.backgroundDataURL = targetDataURL
	err := config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	if err != nil {
		return State{}, err
	}

	s.logf("background appearance updated")
	s.emitState()
	return s.State(), nil
}

func (s *Service) BackgroundDataURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backgroundDataURL
}

func (s *Service) ResetBackground() (State, error) {
	s.mu.Lock()
	currentPath := s.cfg.BackgroundImage
	s.cfg.BackgroundImage = ""
	s.backgroundDataURL = ""
	err := config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	if err != nil {
		return State{}, err
	}

	if strings.TrimSpace(currentPath) != "" &&
		strings.HasPrefix(strings.ToLower(filepath.Base(currentPath)), managedBackgroundBaseName+".") {
		_ = os.Remove(currentPath)
	}

	s.logf("background image cleared")
	s.emitState()
	return s.State(), nil
}

func (s *Service) ResetQuitFlag() State {
	s.mu.Lock()
	s.quitRequested = false
	s.mu.Unlock()
	s.emitState()
	return s.State()
}

func (s *Service) Login(ctx context.Context, account, password string, openBrowser bool) (LoginResult, error) {
	return s.login(ctx, account, password, nil, openBrowser)
}

func (s *Service) LaunchGame() error {
	cfg := s.Config()
	_, err := gameclient.Launch(cfg.GamePath)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.lastAction = "launch_game"
	s.lastError = ""
	s.mu.Unlock()
	s.logf("game client started")
	s.emitState()
	return nil
}

func (s *Service) EnsureCaptchaServer() error {
	return s.startServer()
}

func (s *Service) OpenCaptchaURL() error {
	s.mu.RLock()
	target := s.captchaURL
	s.mu.RUnlock()
	if target == "" {
		return fmt.Errorf("captcha url is empty")
	}
	return browser.OpenURL(target)
}

func (s *Service) EnsureSession(ctx context.Context) error {
	s.mu.RLock()
	ready := s.bhInfo != nil && s.cfg.AccountLogin
	cfg := s.cfg.Clone()
	s.mu.RUnlock()
	if ready {
		return nil
	}
	if cfg.UID == 0 || cfg.AccessKey == "" {
		return fmt.Errorf("missing cached uid/access_key")
	}

	verifyResp, err := s.mihoyo.Verify(ctx, fmt.Sprintf("%d", cfg.UID), cfg.AccessKey)
	if err != nil {
		s.clearCachedSession("cached session verify failed; login required again")
		return err
	}
	if config.Int64Value(verifyResp["retcode"]) != 0 {
		s.clearCachedSession("cached session expired; login required again")
		return fmt.Errorf("verify retcode=%d", config.Int64Value(verifyResp["retcode"]))
	}

	s.mu.Lock()
	s.bhInfo = cloneMap(verifyResp)
	s.cfg.AccountLogin = true
	err = config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	if err != nil {
		return err
	}

	if _, err := s.syncVersionAndDispatch(ctx, false); err != nil {
		s.logf("dispatch refresh skipped after session restore: %v", err)
	}
	s.emitState()
	return nil
}

func (s *Service) clearCachedSession(reason string) {
	s.mu.Lock()
	s.bhInfo = nil
	s.cfg.LastLoginSucc = false
	s.cfg.AccountLogin = false
	s.cfg.UID = 0
	s.cfg.AccessKey = ""
	s.cfg.UName = ""
	saveErr := config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	if saveErr != nil {
		s.logf("clear cached session failed: %v", saveErr)
		return
	}
	if strings.TrimSpace(reason) != "" {
		s.logf("%s", reason)
	}
	s.emitState()
}

func (s *Service) ScanTicket(ctx context.Context, ticket string) (map[string]any, error) {
	if ticket == "" {
		return nil, fmt.Errorf("ticket is required")
	}
	if err := s.EnsureSession(ctx); err != nil {
		return nil, err
	}

	s.mu.RLock()
	bhInfo := cloneMap(s.bhInfo)
	cfg := s.cfg.Clone()
	s.mu.RUnlock()

	result, err := s.mihoyo.ScanCheck(ctx, bhInfo, ticket, cfg)
	if err != nil {
		s.setError(err)
		return nil, err
	}

	s.mu.Lock()
	s.lastTicket = ticket
	s.lastAction = "scan"
	s.lastError = ""
	s.mu.Unlock()

	if config.Int64Value(result["retcode"]) == 0 {
		s.logf("scan confirmed successfully")
		s.mu.RLock()
		autoClose := s.cfg.AutoClose
		s.mu.RUnlock()
		if autoClose {
			s.mu.Lock()
			s.quitRequested = true
			s.lastAction = "quit_requested"
			s.mu.Unlock()
			s.emitState()
		} else {
			s.pauseMonitorAfterSuccess()
		}
		return result, nil
	}

	s.logf("scan did not complete: %v", result)
	if message := strings.TrimSpace(config.StringValue(result["message"])); looksLikeAccessBlock(strings.ToLower(message)) {
		s.setError(fmt.Errorf("scan blocked: %s", message))
	}
	s.emitState()
	return result, nil
}

func (s *Service) ScanURL(ctx context.Context, rawURL string) (map[string]any, error) {
	ticket, err := qr.ExtractTicket(rawURL)
	if err != nil {
		return nil, err
	}
	return s.ScanTicket(ctx, ticket)
}

func (s *Service) ScanClipboardOnce(ctx context.Context) (bool, error) {
	return s.scanClipboardOnce(ctx, false)
}

func (s *Service) ScanWindowOnce(ctx context.Context) (bool, error) {
	return s.scanWindowOnce(ctx, false)
}

func (s *Service) monitorLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		cfg := s.Config()
		if cfg.AutoClip {
			if _, err := s.scanWindowOnce(ctx, true); err != nil {
				s.logf("window capture failed: %v", err)
			}
		}
		if cfg.ClipCheck {
			if _, err := s.scanClipboardOnce(ctx, true); err != nil {
				s.logf("clipboard scan failed: %v", err)
			}
		}

		waitSeconds := s.monitorSleepTime()
		timer := time.NewTimer(time.Duration(waitSeconds) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Service) monitorSleepTime() int {
	s.mu.RLock()
	lastError := strings.ToLower(strings.TrimSpace(s.lastError))
	s.mu.RUnlock()

	if looksLikeAccessBlock(lastError) {
		return blockedSleepTime
	}
	return defaultSleepTime
}

func looksLikeAccessBlock(message string) bool {
	if message == "" {
		return false
	}
	for _, keyword := range []string{
		"429",
		"rate limit",
		"too many",
		"frequency",
		"risk",
		"captcha",
		"拦截",
		"频繁",
		"限制",
	} {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

func (s *Service) startServer() error {
	s.mu.RLock()
	if s.serverStarted {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	if err := s.server.Prepare(); err != nil {
		s.setError(err)
		return err
	}

	s.mu.Lock()
	if s.serverStarted {
		s.mu.Unlock()
		return nil
	}
	s.serverStarted = true
	s.serverReady = true
	s.mu.Unlock()

	go func() {
		err := s.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			s.mu.Lock()
			s.serverReady = false
			s.mu.Unlock()
			s.setError(err)
			s.logf("captcha callback server failed: %v", err)
			s.emitState()
		}
	}()

	s.emitState()
	return nil
}

func (s *Service) initClipboard() {
	s.mu.Lock()
	if s.clipboardReady || s.clipboardErr != nil {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	err := clipboard.Init()

	s.mu.Lock()
	if err == nil {
		s.clipboardReady = true
	} else {
		s.clipboardErr = err
	}
	s.mu.Unlock()

	if err != nil {
		s.logf("clipboard init failed: %v", err)
		return
	}

	data := clipboard.Read(clipboard.FmtImage)
	if len(data) == 0 {
		return
	}

	hash := sha1.Sum(data)
	sum := hex.EncodeToString(hash[:])

	s.mu.Lock()
	s.clipboardHash = sum
	s.mu.Unlock()
}

func (s *Service) monitorReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.AccountLogin && s.bhInfo != nil
}

func (s *Service) prepareScanSession(ctx context.Context, silent bool) error {
	if s.monitorReady() {
		_, _ = s.syncVersionAndDispatch(ctx, false)
		return nil
	}

	if err := s.EnsureSession(ctx); err != nil {
		if silent {
			s.mu.Lock()
			s.lastAction = "waiting_login"
			s.mu.Unlock()
			return nil
		}
		return fmt.Errorf("game session is not ready; login first")
	}

	_, _ = s.syncVersionAndDispatch(ctx, false)
	return nil
}

func (s *Service) scanClipboardOnce(ctx context.Context, silent bool) (bool, error) {
	if err := s.prepareScanSession(ctx, silent); err != nil {
		return false, err
	}

	s.mu.RLock()
	clipboardReady := s.clipboardReady
	clipboardErr := s.clipboardErr
	s.mu.RUnlock()
	if !clipboardReady {
		return false, clipboardErr
	}

	data := clipboard.Read(clipboard.FmtImage)
	if len(data) == 0 {
		return false, nil
	}

	hash := sha1.Sum(data)
	sum := hex.EncodeToString(hash[:])

	if silent {
		s.mu.RLock()
		if s.clipboardHash == sum {
			s.mu.RUnlock()
			return false, nil
		}
		s.mu.RUnlock()
	}

	ticket, rawURL, err := qr.DecodeTicketFromBytes(data)
	if err != nil {
		return false, nil
	}

	s.mu.Lock()
	s.clipboardHash = sum
	s.mu.Unlock()

	return s.consumeTicket(ctx, ticket, rawURL, true)
}

func (s *Service) scanWindowOnce(ctx context.Context, silent bool) (bool, error) {
	if err := s.prepareScanSession(ctx, silent); err != nil {
		return false, err
	}

	window, err := winwindow.FindFirst(targetWindowPattern, targetProcessNames...)
	if err != nil {
		if errors.Is(err, winwindow.ErrTargetWindowNotFound) {
			s.mu.Lock()
			s.lastAction = "waiting_window"
			s.mu.Unlock()
			if !silent {
				s.emitState()
			}
			return false, nil
		}
		return false, err
	}
	if window.Bounds.Empty() || window.Bounds.Dx() <= 0 || window.Bounds.Dy() <= 0 {
		return false, fmt.Errorf("target window bounds are invalid")
	}

	img, err := winwindow.Capture(window)
	if err != nil {
		return false, err
	}

	ok, consumeErr := s.consumeImage(ctx, img, false)
	if consumeErr != nil {
		return false, nil
	}
	return ok, nil
}

func (s *Service) consumeImage(ctx context.Context, img image.Image, clearClipboard bool) (bool, error) {
	ticket, rawURL, err := qr.DecodeTicketFromImage(img)
	if err != nil {
		return false, err
	}
	return s.consumeTicket(ctx, ticket, rawURL, clearClipboard)
}

func (s *Service) consumeTicket(ctx context.Context, ticket, rawURL string, clearClipboard bool) (bool, error) {
	if !s.rememberTicket(ticket) {
		return false, nil
	}

	s.mu.Lock()
	s.lastTicket = ticket
	s.lastQRCodeURL = rawURL
	s.lastAction = "ticket_detected"
	s.lastError = ""
	s.mu.Unlock()

	if _, err := s.ScanTicket(ctx, ticket); err != nil {
		return false, err
	}

	if clearClipboard {
		clipboard.Write(clipboard.FmtImage, nil)
		clipboard.Write(clipboard.FmtText, nil)
	}

	s.emitState()
	return true, nil
}

func (s *Service) rememberTicket(ticket string) bool {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for key, ts := range s.recentTickets {
		if now.Sub(ts) > ticketTTL {
			delete(s.recentTickets, key)
		}
	}

	if ts, ok := s.recentTickets[ticket]; ok && now.Sub(ts) <= ticketTTL {
		return false
	}
	s.recentTickets[ticket] = now
	return true
}

func (s *Service) login(ctx context.Context, account, password string, cap map[string]any, openBrowser bool) (LoginResult, error) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()

	s.mu.Lock()
	if account != "" {
		s.cfg.Account = account
	}
	if password != "" {
		s.cfg.Password = password
	}
	cfg := s.cfg.Clone()
	s.captchaPending = false
	s.captchaURL = ""
	s.lastAction = "login"
	s.lastError = ""
	s.quitRequested = false
	s.mu.Unlock()
	s.emitState()

	result := LoginResult{}

	uid := fmt.Sprintf("%d", cfg.UID)
	accessKey := cfg.AccessKey
	var userInfo map[string]any

	if cfg.LastLoginSucc && cfg.UID != 0 && cfg.AccessKey != "" {
		info, err := s.bili.GetUserInfo(ctx, uid, accessKey)
		if err == nil && config.StringValue(info["uname"]) != "" {
			userInfo = info
			result.UserInfo = info
			result.UName = config.StringValue(info["uname"])
		} else {
			s.mu.Lock()
			s.cfg.LastLoginSucc = false
			s.cfg.UID = 0
			s.cfg.AccessKey = ""
			s.cfg.UName = ""
			saveErr := config.Save(s.cfgPath, s.cfg)
			s.mu.Unlock()
			if saveErr != nil {
				return result, saveErr
			}
			uid = ""
			accessKey = ""
			s.logf("cached bilibili session expired; re-login required")
		}
	}

	if accessKey == "" {
		if account == "" {
			account = cfg.Account
		}
		if password == "" {
			password = cfg.Password
		}
		if account == "" || password == "" {
			return result, fmt.Errorf("account and password are required")
		}

		loginResp, err := s.bili.Login(ctx, account, password, cap)
		if err != nil {
			s.setError(err)
			return result, err
		}
		result.BiliResponse = loginResp

		accessKey = config.StringValue(loginResp["access_key"])
		if accessKey == "" {
			result.Message = config.StringValue(loginResp["message"])
			capData, capErr := s.bili.StartCaptcha(ctx)
			if capErr == nil {
				if err := s.startServer(); err != nil {
					s.setError(err)
					return result, err
				}
				result.CaptchaURL = bsgamesdk.MakeCaptchaURL(
					s.server.Addr(),
					config.StringValue(capData["gt"]),
					config.StringValue(capData["challenge"]),
					config.StringValue(capData["gt_user_id"]),
				)
				result.NeedsCaptcha = result.CaptchaURL != ""
			}

			s.mu.Lock()
			s.captchaPending = result.NeedsCaptcha
			s.captchaURL = result.CaptchaURL
			s.lastAction = "captcha_required"
			s.mu.Unlock()
			s.emitState()

			if result.NeedsCaptcha {
				s.logf("captcha verification is required before login can continue")
				if openBrowser {
					_ = browser.OpenURL(result.CaptchaURL)
				}
				return result, nil
			}

			if result.Message == "" {
				result.Message = "bilibili login failed"
			}
			return result, nil
		}

		uid = config.StringValue(loginResp["uid"])
		if uid == "" {
			err = fmt.Errorf("bilibili login response missing uid")
			s.setError(err)
			return result, err
		}
		info, err := s.bili.GetUserInfo(ctx, uid, accessKey)
		if err != nil {
			s.setError(err)
			return result, err
		}
		userInfo = info
		result.UserInfo = info
		result.UName = config.StringValue(info["uname"])
		if result.UName == "" {
			err = fmt.Errorf("bilibili user info missing uname")
			s.setError(err)
			return result, err
		}

		s.mu.Lock()
		s.cfg.Account = account
		s.cfg.Password = password
		s.cfg.UID = config.Int64Value(uid)
		s.cfg.AccessKey = accessKey
		s.cfg.LastLoginSucc = true
		s.cfg.UName = result.UName
		saveErr := config.Save(s.cfgPath, s.cfg)
		s.mu.Unlock()
		if saveErr != nil {
			return result, saveErr
		}
		s.logf("bilibili login succeeded")
	}

	if userInfo == nil {
		info, err := s.bili.GetUserInfo(ctx, uid, accessKey)
		if err != nil {
			return result, err
		}
		userInfo = info
		result.UserInfo = info
		result.UName = config.StringValue(info["uname"])
		if result.UName == "" {
			err = fmt.Errorf("bilibili user info missing uname")
			s.setError(err)
			return result, err
		}
	}

	verifyResp, err := s.mihoyo.Verify(ctx, uid, accessKey)
	if err != nil {
		s.setError(err)
		return result, err
	}
	result.VerifyResponse = verifyResp
	if config.Int64Value(verifyResp["retcode"]) != 0 {
		err = fmt.Errorf("mihoyo verify retcode=%d", config.Int64Value(verifyResp["retcode"]))
		s.setError(err)
		return result, err
	}

	s.mu.Lock()
	s.bhInfo = cloneMap(verifyResp)
	s.cfg.AccountLogin = true
	saveErr := config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	if saveErr != nil {
		return result, saveErr
	}

	dispatchResp, err := s.syncVersionAndDispatch(ctx, true)
	if err != nil {
		s.setError(err)
		return result, err
	}
	result.DispatchResponse = dispatchResp

	s.mu.Lock()
	s.captchaPending = false
	s.captchaURL = ""
	saveErr = config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	if saveErr != nil {
		return result, saveErr
	}

	result.OK = true
	result.Message = "ok"
	s.logf("login completed")
	s.emitState()
	return result, nil
}

func (s *Service) handleCaptchaResult(payload map[string]any) {
	cfg := s.Config()
	if cfg.Account == "" || cfg.Password == "" {
		s.logf("captcha callback received but account credentials are missing")
		return
	}

	go func(account, password string) {
		if _, err := s.login(context.Background(), account, password, payload, false); err != nil {
			s.logf("captcha login continuation failed: %v", err)
		}
	}(cfg.Account, cfg.Password)
}

func (s *Service) setError(err error) {
	if err == nil {
		return
	}
	message := s.sanitizeMessage(err.Error())
	s.mu.Lock()
	s.lastError = message
	s.mu.Unlock()
	if s.fileLog != nil {
		s.fileLog.Writef("error: %s", message)
	}
	s.emitState()
}

func (s *Service) logf(format string, args ...any) {
	entry := LogEntry{
		At:      time.Now().Format(time.RFC3339Nano),
		Message: s.sanitizeMessage(fmt.Sprintf(format, args...)),
	}

	s.mu.Lock()
	s.logs = append(s.logs, entry)
	if len(s.logs) > maxLogEntries {
		s.logs = append([]LogEntry(nil), s.logs[len(s.logs)-maxLogEntries:]...)
	}
	onLog := s.hooks.OnLog
	s.mu.Unlock()

	if onLog != nil {
		onLog(entry)
	}
	if s.fileLog != nil {
		s.fileLog.Writef("%s", entry.Message)
	}
}

func (s *Service) emitState() {
	s.mu.RLock()
	onState := s.hooks.OnState
	s.mu.RUnlock()
	if onState != nil {
		onState(s.State())
	}
}

func (s *Service) logPath() string {
	if s.fileLog == nil {
		return ""
	}
	return s.fileLog.Path()
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (s *Service) sanitizeMessage(message string) string {
	if message == "" {
		return ""
	}
	for _, rule := range sensitiveLogRules {
		message = rule.pattern.ReplaceAllString(message, rule.replacement)
	}

	s.mu.RLock()
	password := s.cfg.Password
	accessKey := s.cfg.AccessKey
	s.mu.RUnlock()

	for _, secret := range []string{password, accessKey} {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		message = strings.ReplaceAll(message, secret, maskSecret(secret))
	}
	return message
}

func maskSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 4 {
		return strings.Repeat("*", len(secret))
	}
	if len(secret) <= 8 {
		return secret[:1] + strings.Repeat("*", len(secret)-2) + secret[len(secret)-1:]
	}
	return secret[:2] + strings.Repeat("*", len(secret)-4) + secret[len(secret)-2:]
}

func (s *Service) syncGameVersion() (string, bool, error) {
	cfg := s.Config()
	if cfg.GamePath == "" {
		return "", false, nil
	}

	dir, err := gameclient.ResolveDir(cfg.GamePath)
	if err != nil {
		return "", false, err
	}
	version, err := gameclient.ReadVersion(dir)
	if err != nil {
		return "", false, err
	}

	var (
		changed        bool
		versionChanged bool
	)

	s.mu.Lock()
	if s.cfg.GamePath != dir {
		s.cfg.GamePath = dir
		changed = true
	}
	if version != "" && s.cfg.BHVer != version {
		s.cfg.BHVer = version
		s.cfg.DispatchData = ""
		changed = true
		versionChanged = true
	}
	var saveErr error
	if changed {
		saveErr = config.Save(s.cfgPath, s.cfg)
	}
	s.mu.Unlock()
	if saveErr != nil {
		return "", false, saveErr
	}

	if versionChanged {
		s.mihoyo.ResetCache()
		s.logf("detected local BH3 version %s", version)
	}
	if changed {
		s.emitState()
	}
	return version, versionChanged, nil
}

func (s *Service) syncVersionAndDispatch(ctx context.Context, forceDispatch bool) (map[string]any, error) {
	version, versionChanged, err := s.syncGameVersion()
	if err != nil {
		return nil, err
	}

	cfg := s.Config()
	if version == "" {
		version = cfg.BHVer
	}

	if mihoyosdk.LooksLikeFinalDispatch(cfg.DispatchData) {
		resp, _, err := s.useConfiguredDispatch(version)
		if err != nil {
			return nil, err
		}
		s.applyDispatchState(resp)
		if !forceDispatch && !versionChanged {
			return nil, nil
		}
		return resp, nil
	}

	if resp, ok, err := s.activateCachedDispatch(version); err != nil {
		return nil, err
	} else if ok {
		s.applyDispatchState(resp)
		return resp, nil
	}

	return s.refreshDispatchData(ctx)
}

func (s *Service) refreshDispatchData(ctx context.Context) (map[string]any, error) {
	cfg := s.Config()
	openID := s.currentOpenID()
	if openID == "" && cfg != nil {
		openID = cfg.HI3UID
	}
	comboToken := s.currentComboToken()
	if comboToken == "" && cfg != nil {
		comboToken = cfg.BILIHITOKEN
	}
	uid := s.currentUID()
	if uid == "" && cfg != nil {
		uid = cfg.HI3UID
	}
	// If no credential hints at all, skip silent refresh
	if openID == "" && comboToken == "" && uid == "" {
		return nil, nil
	}

	// 自动检测远端包版本并在需要时尝试补全或更新 BILIHITOKEN/BiliPkgVer/BHVer。
	// 行为：
	// - 先尝试从远端获取 GameInfo；若失败则记录日志并跳过。
	// - 若本地缺少 BHVer/BiliPkgVer/BILIHITOKEN 中的任意字段，尝试补全并保存。
	// - 若远端版本与本地不同，记录变更并尝试拉取新的 dispatch token 并持久化。
	if cfg != nil {
		httpClient := netutil.NewClient()
		info, err := bilihitoken.FetchGameInfo(httpClient)
		if err != nil {
			msg := fmt.Sprintf("fetch gameinfo failed: %v", err)
			s.logf("%s", msg)
			if s.fileLog != nil {
				s.fileLog.Writef("%s", msg)
			}
		} else {
			// BHVer 检查与补全
			if strings.TrimSpace(cfg.BHVer) == "" && info.BHVer != "" {
				s.mu.Lock()
				s.cfg.BHVer = info.BHVer
				_ = config.Save(s.cfgPath, s.cfg)
				s.mu.Unlock()
				msg := fmt.Sprintf("filled BHVer=%s from remote", info.BHVer)
				s.logf("%s", msg)
				if s.fileLog != nil {
					s.fileLog.Writef("%s", msg)
				}
			} else if info.BHVer != "" && strings.TrimSpace(cfg.BHVer) != "" && info.BHVer != cfg.BHVer {
				msg := fmt.Sprintf("BHVer changed: %s -> %s", cfg.BHVer, info.BHVer)
				s.logf("%s", msg)
				if s.fileLog != nil {
					s.fileLog.Writef("%s", msg)
				}
				s.mu.Lock()
				s.cfg.BHVer = info.BHVer
				_ = config.Save(s.cfgPath, s.cfg)
				s.mu.Unlock()
			} else if info.BHVer != "" {
				msg := fmt.Sprintf("BHVer consistent: %s", cfg.BHVer)
				s.logf("%s", msg)
				if s.fileLog != nil {
					s.fileLog.Writef("%s", msg)
				}
			}

			// BiliPkgVer 检查与补全
			if cfg.BiliPkgVer == 0 {
				if info.Version != 0 {
					s.mu.Lock()
					s.cfg.BiliPkgVer = info.Version
					_ = config.Save(s.cfgPath, s.cfg)
					s.mu.Unlock()
					msg := fmt.Sprintf("filled BiliPkgVer=%d from remote", info.Version)
					s.logf("%s", msg)
					if s.fileLog != nil {
						s.fileLog.Writef("%s", msg)
					}
				} else {
					msg := "remote BiliPkgVer unknown; local is 0"
					s.logf("%s", msg)
					if s.fileLog != nil {
						s.fileLog.Writef("%s", msg)
					}
				}
			} else if info.Version != 0 && info.Version == cfg.BiliPkgVer {
				msg := fmt.Sprintf("BiliPkgVer consistent: %d", cfg.BiliPkgVer)
				s.logf("%s", msg)
				if s.fileLog != nil {
					s.fileLog.Writef("%s", msg)
				}
			} else if info.Version != 0 && info.Version != cfg.BiliPkgVer {
				msg := fmt.Sprintf("BiliPkgVer changed: %d -> %d", cfg.BiliPkgVer, info.Version)
				s.logf("%s", msg)
				// 远端版本变化，尝试获取新的 token（仅在能成功获取时保存）
				if s.fileLog != nil {
					s.fileLog.Writef("%s", msg)
				}
				token, err := bilihitoken.FetchDispatchToken(httpClient, info.APKUrl)
				if err == nil && strings.TrimSpace(token) != "" {
					s.mu.Lock()
					s.cfg.BILIHITOKEN = token
					s.cfg.BiliPkgVer = info.Version
					if info.BHVer != "" {
						s.cfg.BHVer = info.BHVer
					}
					_ = config.Save(s.cfgPath, s.cfg)
					s.mu.Unlock()
					comboToken = s.cfg.BILIHITOKEN
					msg2 := fmt.Sprintf("auto-updated BILIHITOKEN to pkg_ver=%d", info.Version)
					s.logf("%s", msg2)
					if s.fileLog != nil {
						s.fileLog.Writef("%s", msg2)
					}
				} else {
					msg2 := fmt.Sprintf("auto-update BILIHITOKEN failed: %v", err)
					s.logf("%s", msg2)
					if s.fileLog != nil {
						s.fileLog.Writef("%s", msg2)
					}
					s.mu.Lock()
					s.lastError = "自动更新 BILIHITOKEN 失败，请手动获取"
					s.mu.Unlock()
					s.emitState()
				}
			}

			// 若本地缺少 BILIHITOKEN，尝试补全（在有 remote info 且已设置 game path 时尝试）
			if strings.TrimSpace(cfg.BILIHITOKEN) == "" {
				if strings.TrimSpace(cfg.GamePath) != "" && info.Version != 0 {
					token, err := bilihitoken.FetchDispatchToken(httpClient, info.APKUrl)
					if err == nil && strings.TrimSpace(token) != "" {
						s.mu.Lock()
						s.cfg.BILIHITOKEN = token
						if info.Version != 0 {
							s.cfg.BiliPkgVer = info.Version
						}
						if info.BHVer != "" {
							s.cfg.BHVer = info.BHVer
						}
						_ = config.Save(s.cfgPath, s.cfg)
						s.mu.Unlock()
						comboToken = s.cfg.BILIHITOKEN
						msg := fmt.Sprintf("filled missing BILIHITOKEN via remote fetch; pkg_ver=%d", info.Version)
						s.logf("%s", msg)
						if s.fileLog != nil {
							s.fileLog.Writef("%s", msg)
						}
					} else {
						msg := fmt.Sprintf("failed to fill missing BILIHITOKEN: %v", err)
						s.logf("%s", msg)
						if s.fileLog != nil {
							s.fileLog.Writef("%s", msg)
						}
					}
				} else {
					msg := "BILIHITOKEN missing and GamePath not set or remote version unknown; skipping auto-fill"
					s.logf("%s", msg)
					if s.fileLog != nil {
						s.fileLog.Writef("%s", msg)
					}
				}
			}
		}
	}

	s.mihoyo.ResetDispatchCache()

	resp, err := s.mihoyo.GetOAServer(ctx, openID, comboToken, uid, cfg)
	if err != nil {
		return nil, err
	}
	if retcode := config.Int64Value(resp["retcode"]); retcode != 0 && resp["retcode"] != nil {
		return resp, fmt.Errorf("dispatch retcode=%d", retcode)
	}

	dispatchData := config.StringValue(resp["data"])
	if dispatchData == "" {
		return resp, fmt.Errorf("dispatch response missing data")
	}
	if !mihoyosdk.LooksLikeFinalDispatch(dispatchData) {
		return resp, fmt.Errorf("dispatch response is not a usable final blob")
	}

	entry := buildDispatchCacheEntry(resp)
	cacheKey := dispatchCacheKey(cfg.BHVer)

	s.mu.Lock()
	s.cfg.DispatchData = dispatchData
	if s.cfg.DispatchCache == nil {
		s.cfg.DispatchCache = map[string]config.DispatchCacheEntry{}
	}
	if cacheKey != "" {
		s.cfg.DispatchCache[cacheKey] = entry
		s.cfg.DispatchCache = trimDispatchCache(s.cfg.DispatchCache)
	}
	saveErr := config.Save(s.cfgPath, s.cfg)
	currentVersion := s.cfg.BHVer
	s.mu.Unlock()
	if saveErr != nil {
		return resp, saveErr
	}

	s.applyDispatchState(resp)

	source := config.StringValue(resp["source"])
	requestMode := config.StringValue(resp["request_mode"])
	if source == "" {
		source = "unknown"
	}
	if requestMode != "" {
		s.logf("dispatch refreshed for version %s via %s", currentVersion, source)
	} else {
		s.logf("dispatch refreshed for version %s via %s", currentVersion, source)
	}
	s.emitState()
	return resp, nil
}

// ManualRefreshDispatch sets HI3UID and BILIHITOKEN in config and triggers a dispatch refresh.
func (s *Service) ManualRefreshDispatch(ctx context.Context, hi3uid, biliHitoken string) (State, error) {
	s.mu.Lock()
	s.cfg.HI3UID = strings.TrimSpace(hi3uid)
	s.cfg.BILIHITOKEN = strings.TrimSpace(biliHitoken)
	if err := config.Save(s.cfgPath, s.cfg); err != nil {
		s.mu.Unlock()
		return s.State(), err
	}
	s.mu.Unlock()

	// Attempt refresh
	_, err := s.refreshDispatchData(ctx)
	s.emitState()
	return s.State(), err
}

// ManualFetchBiliHitoken attempts to fetch a fresh BILIHITOKEN from the remote APK
// and saves it into config. Requires game path to be set locally.
func (s *Service) ManualFetchBiliHitoken(ctx context.Context) (State, error) {
	cfg := s.Config()
	if cfg == nil || strings.TrimSpace(cfg.GamePath) == "" {
		return s.State(), fmt.Errorf("game path is not set")
	}

	httpClient := netutil.NewClient()
	info, err := bilihitoken.FetchGameInfo(httpClient)
	if err != nil {
		return s.State(), err
	}
	token, err := bilihitoken.FetchDispatchToken(httpClient, info.APKUrl)
	if err != nil {
		return s.State(), err
	}
	if strings.TrimSpace(token) == "" {
		return s.State(), fmt.Errorf("fetched empty BILIHITOKEN")
	}

	s.mu.Lock()
	s.cfg.BILIHITOKEN = token
	s.cfg.BiliPkgVer = info.Version
	if info.BHVer != "" {
		s.cfg.BHVer = info.BHVer
	}
	saveErr := config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	if saveErr != nil {
		return s.State(), saveErr
	}

	s.logf("manually fetched BILIHITOKEN pkg_ver=%d", info.Version)
	s.emitState()
	return s.State(), nil
}

func (s *Service) useConfiguredDispatch(version string) (map[string]any, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !mihoyosdk.LooksLikeFinalDispatch(s.cfg.DispatchData) {
		return nil, false, nil
	}
	officialMode := strings.TrimSpace(s.cfg.DispatchAPI) == "" || strings.Contains(strings.ToLower(strings.TrimSpace(s.cfg.DispatchAPI)), "query_gameserver") || strings.Contains(strings.ToLower(strings.TrimSpace(s.cfg.DispatchAPI)), "outer-dp-bb01.bh3.com")
	if officialMode {
		// In official mode, always prefer official request path over static dispatch_data.
		return nil, false, nil
	}

	key := dispatchCacheKey(version)
	if key == "" {
		return nil, false, nil
	}

	if s.cfg.DispatchCache == nil {
		s.cfg.DispatchCache = map[string]config.DispatchCacheEntry{}
	}

	entry, ok := s.cfg.DispatchCache[key]
	changed := false
	if !ok || entry.Data != s.cfg.DispatchData {
		entry = buildDispatchCacheEntry(map[string]any{
			"data":   s.cfg.DispatchData,
			"source": "local_config",
		})
		s.cfg.DispatchCache[key] = entry
		s.cfg.DispatchCache = trimDispatchCache(s.cfg.DispatchCache)
		changed = true
	}

	if changed {
		if err := config.Save(s.cfgPath, s.cfg); err != nil {
			return nil, false, err
		}
	}

	return dispatchResponseFromCache(key, entry, "local_config"), true, nil
}

func (s *Service) activateCachedDispatch(version string) (map[string]any, bool, error) {
	key := dispatchCacheKey(version)
	if key == "" {
		return nil, false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.cfg.DispatchCache[key]
	if !ok || !mihoyosdk.LooksLikeFinalDispatch(entry.Data) {
		return nil, false, nil
	}
	officialMode := strings.TrimSpace(s.cfg.DispatchAPI) == "" || strings.Contains(strings.ToLower(strings.TrimSpace(s.cfg.DispatchAPI)), "query_gameserver") || strings.Contains(strings.ToLower(strings.TrimSpace(s.cfg.DispatchAPI)), "outer-dp-bb01.bh3.com")
	if officialMode && shouldSkipOfficialCacheSource(entry.Source) {
		return nil, false, nil
	}

	changed := false
	if s.cfg.DispatchData != entry.Data {
		s.cfg.DispatchData = entry.Data
		changed = true
	}
	if changed {
		if err := config.Save(s.cfgPath, s.cfg); err != nil {
			return nil, false, err
		}
	}

	return dispatchResponseFromCache(key, entry, "local_cache"), true, nil
}

func dispatchCacheKey(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if strings.Contains(version, "_gf_") {
		return version
	}
	return version + "_gf_android_bilibili"
}

func dispatchResponseFromCache(key string, entry config.DispatchCacheEntry, source string) map[string]any {
	resp := map[string]any{
		"retcode":   0,
		"message":   "OK",
		"data":      entry.Data,
		"source":    source,
		"cache_key": key,
	}

	if entry.Source != "" {
		resp["cached_source"] = entry.Source
	}
	if entry.SavedAt != "" {
		resp["cached_saved_at"] = entry.SavedAt
	}

	summary := map[string]any{}
	if entry.RawLen > 0 {
		summary["raw_len"] = entry.RawLen
	}
	if entry.DecodedLen > 0 {
		summary["decoded_len"] = entry.DecodedLen
	}
	if entry.DecodedSHA256 != "" {
		summary["decoded_sha256"] = entry.DecodedSHA256
	}
	if len(summary) > 0 {
		resp["blob_summary"] = summary
	}

	return resp
}

func buildDispatchCacheEntry(resp map[string]any) config.DispatchCacheEntry {
	entry := config.DispatchCacheEntry{
		Data:    config.StringValue(resp["data"]),
		Source:  config.StringValue(resp["source"]),
		SavedAt: time.Now().Format(time.RFC3339),
	}

	if summary, ok := resp["blob_summary"].(map[string]any); ok {
		entry.RawLen = int(config.Int64Value(summary["raw_len"]))
		entry.DecodedLen = int(config.Int64Value(summary["decoded_len"]))
		entry.DecodedSHA256 = config.StringValue(summary["decoded_sha256"])
	}

	return entry
}

func trimDispatchCache(cache map[string]config.DispatchCacheEntry) map[string]config.DispatchCacheEntry {
	if len(cache) <= maxDispatchCaches {
		return cache
	}

	type cacheItem struct {
		key     string
		savedAt string
	}

	items := make([]cacheItem, 0, len(cache))
	for key, entry := range cache {
		items = append(items, cacheItem{key: key, savedAt: entry.SavedAt})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].savedAt == items[j].savedAt {
			return items[i].key < items[j].key
		}
		return items[i].savedAt < items[j].savedAt
	})

	trimmed := make(map[string]config.DispatchCacheEntry, maxDispatchCaches)
	for _, item := range items[len(items)-maxDispatchCaches:] {
		trimmed[item.key] = cache[item.key]
	}
	return trimmed
}

func (s *Service) applyDispatchState(resp map[string]any) {
	source := strings.TrimSpace(config.StringValue(resp["source"]))
	if source == "" {
		source = "unknown"
	}
	if cachedSource := strings.TrimSpace(config.StringValue(resp["cached_source"])); cachedSource != "" {
		source = source + ":" + cachedSource
	}

	s.mu.Lock()
	s.dispatchSource = source
	s.mu.Unlock()
	s.emitState()
}

func (s *Service) logAvailableDispatchSource() {
	cfg := s.Config()
	version := strings.TrimSpace(cfg.BHVer)
	if version == "" {
		return
	}

	if mihoyosdk.LooksLikeFinalDispatch(cfg.DispatchData) {
		resp, ok, err := s.useConfiguredDispatch(version)
		if err == nil && ok {
			s.applyDispatchState(resp)
			return
		}
	}

	if resp, ok, err := s.activateCachedDispatch(version); err == nil && ok {
		s.applyDispatchState(resp)
	}
}

func (s *Service) currentOpenID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return openIDFromVerifyResponse(s.bhInfo)
}

func (s *Service) currentComboToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return comboTokenFromVerifyResponse(s.bhInfo)
}

func (s *Service) currentUID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	uid := uidFromVerifyResponse(s.bhInfo)
	if uid != "" {
		return uid
	}
	if s.cfg != nil && s.cfg.UID != 0 {
		return fmt.Sprintf("%d", s.cfg.UID)
	}
	return ""
}

func openIDFromVerifyResponse(resp map[string]any) string {
	if resp == nil {
		return ""
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		return ""
	}
	return config.StringValue(data["open_id"])
}

func comboTokenFromVerifyResponse(resp map[string]any) string {
	if resp == nil {
		return ""
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		return ""
	}
	return config.StringValue(data["combo_token"])
}

func uidFromVerifyResponse(resp map[string]any) string {
	if resp == nil {
		return ""
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		return ""
	}
	return config.StringValue(data["uid"])
}

func shouldSkipOfficialCacheSource(source string) bool {
	source = strings.TrimSpace(strings.ToLower(source))
	return source == "" || source == "reference_third_party" || source == "custom_dispatch_api"
}

func clampBackgroundOpacity(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func samePath(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return left == right
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil && rightErr == nil {
		return strings.EqualFold(leftAbs, rightAbs)
	}
	return strings.EqualFold(left, right)
}

func managedBackgroundPathFor(sourcePath string) (string, error) {
	// Store managed background in the executable directory so packaged apps carry it alongside the binary.
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(sourcePath)))
	if ext == "" {
		ext = ".img"
	}
	return filepath.Join(exeDir, managedBackgroundBaseName+ext), nil
}

func loadBackgroundDataURL(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	// Try as-given first
	data, err := os.ReadFile(path)
	if err != nil {
		// If path is not absolute, try resolving relative to executable directory
		if !filepath.IsAbs(path) {
			exePath, eerr := os.Executable()
			if eerr == nil {
				alt := filepath.Join(filepath.Dir(exePath), path)
				data, err = os.ReadFile(alt)
			}
		}
		if err != nil {
			return "", fmt.Errorf("read background image: %w", err)
		}
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return "", fmt.Errorf("selected file is not an image")
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func (s *Service) copyManagedBackground(sourcePath, previousPath string) (string, string, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "", "", nil
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", "", fmt.Errorf("read background image: %w", err)
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(sourcePath)))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return "", "", fmt.Errorf("selected file is not an image")
	}

	destAbsPath, err := managedBackgroundPathFor(sourcePath)
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(destAbsPath, data, 0o644); err != nil {
		return "", "", fmt.Errorf("write background image: %w", err)
	}

	if previousPath != "" && !samePath(previousPath, destAbsPath) &&
		strings.HasPrefix(strings.ToLower(filepath.Base(previousPath)), managedBackgroundBaseName+".") {
		_ = os.Remove(previousPath)
	}

	sum := sha256.Sum256(data)
	s.logf("background image copied to %s (%s)", destAbsPath, hex.EncodeToString(sum[:8]))

	// Store relative path in config for portability (relative to executable dir)
	rel := "./" + filepath.Base(destAbsPath)
	return rel, "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func evaluateGamePath(path string) (bool, string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return false, "未设置游戏目录，请浏览选择崩坏3安装目录。"
	}
	if _, err := gameclient.ResolveDir(path); err != nil {
		return false, "游戏目录无效，请重新浏览选择崩坏3安装目录。"
	}
	return true, ""
}
