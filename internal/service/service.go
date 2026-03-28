package service

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"hi3loader/internal/bilihitoken"
	"hi3loader/internal/bridge"
	"hi3loader/internal/bsgamesdk"
	"hi3loader/internal/captcha"
	"hi3loader/internal/config"
	"hi3loader/internal/debuglog"
	"hi3loader/internal/gameclient"
	"hi3loader/internal/mihoyosdk"
	"hi3loader/internal/netutil"
	"hi3loader/internal/qr"
	"hi3loader/internal/winwindow"

	purewebp "github.com/deepteams/webp"
	"github.com/pkg/browser"
	"golang.design/x/clipboard"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
)

const (
	defaultConfigPath         = "config.json"
	maxLogEntries             = 300
	ticketTTL                 = 30 * time.Second
	pendingCredentialTTL      = 10 * time.Minute
	defaultSleepTime          = 3
	blockedSleepTime          = 8
	manualWindowScanAttempts  = 1
	manualWindowScanDelay     = 250 * time.Millisecond
	windowMissBackoffSeconds  = 6
	windowMissMaxSeconds      = 10
	windowStaticSkipThreshold = 2
	windowStaticDecodePeriod  = 3
	managedBackgroundBaseName = "custom_background"
	managedBackgroundExt      = ".webp"
)

var (
	targetWindowPattern  = regexp.MustCompile(`\x{5D29}\x{574F}3`)
	targetProcessNames   = []string{"bh3.exe"}
	versionStringPattern = regexp.MustCompile(`\d+(?:\.\d+)+`)
	sensitiveLogRules    = []struct {
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
	Config           ConfigView `json:"config"`
	Running          bool       `json:"running"`
	ServerAddress    string     `json:"serverAddress"`
	ServerReady      bool       `json:"serverReady"`
	DispatchSource   string     `json:"dispatchSource"`
	GamePathValid    bool       `json:"gamePathValid"`
	GamePathPrompt   string     `json:"gamePathPrompt"`
	GamePathMessage  MessageRef `json:"gamePathMessage"`
	LogPath          string     `json:"logPath"`
	CaptchaURL       string     `json:"captchaURL"`
	CaptchaPending   bool       `json:"captchaPending"`
	LastAction       string     `json:"lastAction"`
	LastError        string     `json:"lastError"`
	LastErrorMessage MessageRef `json:"lastErrorMessage"`
	LastTicket       string     `json:"lastTicket"`
	LastQRCodeURL    string     `json:"lastQRCodeURL"`
	QuitRequested    bool       `json:"quitRequested"`
}

type LoginResult struct {
	OK             bool              `json:"ok"`
	NeedsCaptcha   bool              `json:"needsCaptcha"`
	CaptchaURL     string            `json:"captchaURL,omitempty"`
	Message        string            `json:"message,omitempty"`
	MessageCode    string            `json:"messageCode,omitempty"`
	MessageParams  map[string]string `json:"messageParams,omitempty"`
	UName          string            `json:"uname,omitempty"`
	SessionReady   bool              `json:"sessionReady,omitempty"`
	DispatchReady  bool              `json:"dispatchReady,omitempty"`
	DispatchSource string            `json:"dispatchSource,omitempty"`
	Retcode        int64             `json:"retcode,omitempty"`
}

type ScanResult struct {
	OK            bool              `json:"ok"`
	Confirmed     bool              `json:"confirmed"`
	Message       string            `json:"message,omitempty"`
	MessageCode   string            `json:"messageCode,omitempty"`
	MessageParams map[string]string `json:"messageParams,omitempty"`
	Retcode       int64             `json:"retcode,omitempty"`
	QuitRequested bool              `json:"quitRequested,omitempty"`
}

type ScanWindowResult struct {
	Matched    bool       `json:"matched"`
	Message    string     `json:"message,omitempty"`
	MessageRef MessageRef `json:"messageRef"`
}

type Hooks struct {
	OnLog   func(LogEntry)
	OnState func(State)
}

type Service struct {
	cfgPath string

	bili   *bsgamesdk.Client
	mihoyo *mihoyosdk.Client
	bridge *bridge.Client

	mu                 sync.RWMutex
	cfg                *config.Config
	server             *captcha.Server
	serverReady        bool
	serverStarted      bool
	running            bool
	loopCancel         context.CancelFunc
	monitorWake        chan struct{}
	captchaURL         string
	captchaPending     bool
	dispatchSource     string
	lastAction         string
	lastError          string
	lastErrorMessage   MessageRef
	lastNoticeCode     string
	lastNoticeAt       time.Time
	lastTicket         string
	lastQRCodeURL      string
	quitRequested      bool
	logs               []LogEntry
	sessionInfo        *mihoyosdk.SessionInfo
	backgroundDataURL  string
	recentTickets      map[string]time.Time
	clipboardHash      string
	clipboardReady     bool
	clipboardErr       error
	windowMissStreak   int
	windowStaticStreak int
	windowFingerprint  string
	hooks              Hooks
	fileLog            *debuglog.Logger
	loginMu            sync.Mutex
	pendingAccount     string
	pendingPassword    []byte
	pendingPasswordTTL time.Time
	pendingPasswordGen uint64
	pendingPasswordTmr *time.Timer
}

type Options struct {
	BridgeExecutable string
	RequireBridge    bool
}

func New(cfgPath string) (*Service, error) {
	return NewWithOptions(cfgPath, Options{})
}

func NewWithOptions(cfgPath string, opts Options) (*Service, error) {
	if cfgPath == "" {
		cfgPath = defaultConfigPath
	}

	if !filepath.IsAbs(cfgPath) {
		absPath, err := filepath.Abs(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("resolve config path: %w", err)
		}
		cfgPath = absPath
	}

	cfg, err := config.LoadOrCreate(cfgPath)
	if err != nil {
		return nil, err
	}

	logDir := filepath.Join(filepath.Dir(cfgPath), "logs")
	fileLog, _ := debuglog.New(logDir, "hi3loader-debug.log")

	bridgeClient, err := bridge.NewClient(opts.BridgeExecutable)
	if err != nil {
		return nil, fmt.Errorf("init helper bridge: %w", err)
	}
	if opts.RequireBridge && bridgeClient == nil {
		return nil, fmt.Errorf("helper bridge is required in this mode")
	}

	s := &Service{
		cfgPath:       cfgPath,
		cfg:           cfg,
		bili:          bsgamesdk.NewClient(),
		mihoyo:        mihoyosdk.NewClient(),
		bridge:        bridgeClient,
		fileLog:       fileLog,
		monitorWake:   make(chan struct{}, 1),
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

	_, _, _, _ = s.syncGameVersion()
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
	lastErrorMessage := cloneMessageRef(s.lastErrorMessage)
	lastTicket := s.lastTicket
	lastQRCodeURL := s.lastQRCodeURL
	quitRequested := s.quitRequested
	s.mu.RUnlock()

	gamePathValid, gamePathMessage := evaluateGamePath(cfg.GamePath)
	gamePathPrompt := fallbackMessageText(gamePathMessage)

	return State{
		Config:           buildConfigView(cfg),
		Running:          running,
		ServerAddress:    s.server.Addr(),
		ServerReady:      serverReady,
		DispatchSource:   dispatchSource,
		GamePathValid:    gamePathValid,
		GamePathPrompt:   gamePathPrompt,
		GamePathMessage:  gamePathMessage,
		LogPath:          logPath,
		CaptchaURL:       captchaURL,
		CaptchaPending:   captchaPending,
		LastAction:       lastAction,
		LastError:        lastError,
		LastErrorMessage: lastErrorMessage,
		LastTicket:       lastTicket,
		LastQRCodeURL:    lastQRCodeURL,
		QuitRequested:    quitRequested,
	}
}

func (s *Service) LogSnapshot() []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]LogEntry(nil), s.logs...)
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

	if _, _, _, err := s.syncGameVersion(); err != nil {
		return State{}, err
	}
	_, _ = s.syncVersionAndDispatch(context.Background(), false)
	s.emitState()
	if autoClip || clipCheck {
		s.wakeMonitorLoop()
	}
	return s.State(), nil
}

// SaveSetting updates a single config field and persists the config.
func (s *Service) SaveSetting(key string, value any) (State, error) {
	key = strings.TrimSpace(key)

	s.mu.Lock()
	if s.cfg == nil {
		s.mu.Unlock()
		return s.State(), fmt.Errorf("config is not loaded")
	}
	oldVal := settingAuditValue(s.cfg, key)
	nextCfg := s.cfg.Clone()
	if err := applySettingValue(nextCfg, key, value); err != nil {
		s.mu.Unlock()
		return s.State(), err
	}
	newVal := settingAuditValue(nextCfg, key)
	err := config.Save(s.cfgPath, nextCfg)
	if err == nil {
		s.cfg = nextCfg
	}
	s.mu.Unlock()

	if s.fileLog != nil {
		if err != nil {
			s.fileLog.Writef("save setting failed: %s '%s' -> '%s': %v", key, oldVal, newVal, err)
		} else {
			s.fileLog.Writef("save setting: %s '%s' -> '%s'", key, oldVal, newVal)
		}
	}
	if err != nil {
		return s.State(), err
	}

	s.emitState()
	switch key {
	case "auto_clip", "clip_check", "game_path":
		s.wakeMonitorLoop()
	}
	return s.State(), nil
}

func (s *Service) RecordClientMessage(message string) {
	message = strings.TrimSpace(s.sanitizeMessage(message))
	if message == "" || s.fileLog == nil {
		return
	}
	s.fileLog.Writef("%s", message)
}

func applySettingValue(cfg *config.Config, key string, value any) error {
	switch key {
	case "account":
		cfg.Account = strings.TrimSpace(fmt.Sprintf("%v", value))
	case "password":
		cfg.Password = fmt.Sprintf("%v", value)
	case "HI3UID":
		cfg.HI3UID = strings.TrimSpace(fmt.Sprintf("%v", value))
	case "BILIHITOKEN":
		cfg.BILIHITOKEN = strings.TrimSpace(fmt.Sprintf("%v", value))
	case "panel_blur":
		b, ok := value.(bool)
		if !ok {
			return fmt.Errorf("setting %s expects bool", key)
		}
		cfg.PanelBlur = b
	case "clip_check":
		b, ok := value.(bool)
		if !ok {
			return fmt.Errorf("setting %s expects bool", key)
		}
		cfg.ClipCheck = b
	case "auto_clip":
		b, ok := value.(bool)
		if !ok {
			return fmt.Errorf("setting %s expects bool", key)
		}
		cfg.AutoClip = b
	case "auto_close":
		b, ok := value.(bool)
		if !ok {
			return fmt.Errorf("setting %s expects bool", key)
		}
		cfg.AutoClose = b
	case "background_opacity":
		f, ok := value.(float64)
		if !ok {
			return fmt.Errorf("setting %s expects number", key)
		}
		cfg.BackgroundOpacity = clampBackgroundOpacity(f)
	case "game_path":
		cfg.GamePath = strings.TrimSpace(fmt.Sprintf("%v", value))
	default:
		return fmt.Errorf("unknown setting: %s", key)
	}
	return nil
}

func settingAuditValue(cfg *config.Config, key string) string {
	if cfg == nil {
		return "<nil>"
	}

	switch key {
	case "account":
		return maskSecret(cfg.Account)
	case "password":
		return "<redacted>"
	case "HI3UID":
		return maskSecret(cfg.HI3UID)
	case "BILIHITOKEN":
		return maskSecret(cfg.BILIHITOKEN)
	case "panel_blur":
		return fmt.Sprintf("%v", cfg.PanelBlur)
	case "clip_check":
		return fmt.Sprintf("%v", cfg.ClipCheck)
	case "auto_clip":
		return fmt.Sprintf("%v", cfg.AutoClip)
	case "auto_close":
		return fmt.Sprintf("%v", cfg.AutoClose)
	case "background_opacity":
		return fmt.Sprintf("%v", cfg.BackgroundOpacity)
	case "game_path":
		return cfg.GamePath
	default:
		return "<unknown>"
	}
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
	s.lastErrorMessage = MessageRef{}
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
		return localizedErrorf("backend.error.captcha_url_empty", nil, "captcha url is empty")
	}
	return browser.OpenURL(target)
}

func (s *Service) EnsureSession(ctx context.Context) error {
	s.mu.RLock()
	ready := s.sessionInfo != nil && s.cfg.AccountLogin
	cfg := s.cfg.Clone()
	s.mu.RUnlock()
	if ready {
		return nil
	}
	if cfg.AccessKey == "" {
		return fmt.Errorf("missing cached access_key")
	}

	if s.bridge != nil {
		verifyResp, err := s.bridge.VerifySession(ctx, bridge.VerifyRequest{
			UID:       cfg.UID,
			AccessKey: cfg.AccessKey,
		})
		if err != nil {
			s.clearCachedSession("cached session verify failed; login required again")
			return err
		}
		if verifyResp.Retcode != 0 {
			s.clearCachedSession("cached session expired; login required again")
			return localizedErrorf(
				"backend.error.verify_retcode",
				map[string]string{
					"source":  "Cached session",
					"retcode": strconv.FormatInt(verifyResp.Retcode, 10),
				},
				"verify retcode=%d",
				verifyResp.Retcode,
			)
		}

		s.mu.Lock()
		s.sessionInfo = cloneSessionInfo(&verifyResp.Session)
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

	verifyUID := ""
	if cfg.UID != 0 {
		verifyUID = fmt.Sprintf("%d", cfg.UID)
	}
	verifyResp, err := s.mihoyo.Verify(ctx, verifyUID, cfg.AccessKey)
	if err != nil {
		s.clearCachedSession("cached session verify failed; login required again")
		return err
	}
	if config.Int64Value(verifyResp["retcode"]) != 0 {
		s.clearCachedSession("cached session expired; login required again")
		return localizedErrorf(
			"backend.error.verify_retcode",
			map[string]string{
				"source":  "Cached session",
				"retcode": strconv.FormatInt(config.Int64Value(verifyResp["retcode"]), 10),
			},
			"verify retcode=%d",
			config.Int64Value(verifyResp["retcode"]),
		)
	}

	session, err := mihoyosdk.ExtractSessionInfo(verifyResp)
	if err != nil {
		s.setError(err)
		return err
	}

	s.mu.Lock()
	s.sessionInfo = cloneSessionInfo(session)
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
	s.sessionInfo = nil
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

func (s *Service) ScanTicket(ctx context.Context, ticket string) (ScanResult, error) {
	if ticket == "" {
		return ScanResult{}, localizedErrorf("backend.error.ticket_required", nil, "ticket is required")
	}
	if err := s.EnsureSession(ctx); err != nil {
		return ScanResult{}, err
	}

	s.mu.RLock()
	sessionInfo := cloneSessionInfo(s.sessionInfo)
	cfg := s.cfg.Clone()
	s.mu.RUnlock()

	if sessionInfo == nil {
		return ScanResult{}, localizedErrorf("backend.error.session_not_ready", nil, "session is not ready")
	}

	var (
		result map[string]any
		err    error
	)
	if s.bridge != nil {
		scanResp, scanErr := s.bridge.ScanCheck(ctx, bridge.ScanRequest{
			Config:  bridgeConfigSnapshot(cfg),
			Session: *sessionInfo,
			Ticket:  ticket,
		})
		if scanErr != nil {
			s.setError(scanErr)
			return ScanResult{}, scanErr
		}
		result = map[string]any{
			"retcode": scanResp.Retcode,
			"message": scanResp.Message,
		}
	} else {
		result, err = s.mihoyo.ScanCheck(ctx, *sessionInfo, ticket, cfg)
	}
	if err != nil {
		s.setError(err)
		return ScanResult{}, err
	}

	s.mu.Lock()
	s.lastTicket = ticket
	s.lastAction = "scan"
	s.lastError = ""
	s.lastErrorMessage = MessageRef{}
	s.mu.Unlock()

	if config.Int64Value(result["retcode"]) == 0 {
		s.logf("scan confirmed successfully")
		s.mu.RLock()
		autoClose := s.cfg.AutoClose
		s.mu.RUnlock()
		outcome := summarizeScanResult(result)
		if autoClose {
			s.mu.Lock()
			s.quitRequested = true
			s.lastAction = "quit_requested"
			s.mu.Unlock()
			s.emitState()
			outcome.QuitRequested = true
		} else {
			s.pauseMonitorAfterSuccess()
		}
		return outcome, nil
	}

	s.logf("scan did not complete retcode=%d message=%s", config.Int64Value(result["retcode"]), sanitizeResultMessage(result))
	if message := strings.TrimSpace(config.StringValue(result["message"])); looksLikeAccessBlock(strings.ToLower(message)) {
		s.setError(localizedErrorf("backend.error.scan_blocked", map[string]string{"reason": message}, "scan blocked: %s", message))
	}
	s.emitState()
	return summarizeScanResult(result), nil
}

func (s *Service) ScanURL(ctx context.Context, rawURL string) (ScanResult, error) {
	ticket, err := qr.ExtractTicket(rawURL)
	if err != nil {
		return ScanResult{}, err
	}
	return s.ScanTicket(ctx, ticket)
}

func (s *Service) ScanClipboardOnce(ctx context.Context) (bool, error) {
	return s.scanClipboardOnce(ctx, false)
}

func (s *Service) ScanWindow(ctx context.Context) (ScanWindowResult, error) {
	for attempt := 0; attempt < manualWindowScanAttempts; attempt++ {
		matched, hint, err := s.scanWindowOnce(ctx, false)
		if err != nil {
			return ScanWindowResult{}, err
		}
		if matched {
			return ScanWindowResult{Matched: true}, nil
		}
		if hint.Code != "" {
			return newScanWindowResult(false, hint), nil
		}
		if attempt+1 < manualWindowScanAttempts {
			time.Sleep(manualWindowScanDelay)
		}
	}
	return ScanWindowResult{Matched: false}, nil
}

func (s *Service) ScanWindowOnce(ctx context.Context) (bool, error) {
	matched, _, err := s.scanWindowOnce(ctx, false)
	return matched, err
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
			if _, _, err := s.scanWindowOnce(ctx, true); err != nil {
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
		case <-s.monitorWake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
		}
	}
}

func (s *Service) wakeMonitorLoop() {
	s.mu.RLock()
	running := s.running
	s.mu.RUnlock()
	if !running {
		return
	}
	select {
	case s.monitorWake <- struct{}{}:
	default:
	}
}

func (s *Service) monitorSleepTime() int {
	s.mu.RLock()
	sleepTime := s.cfg.SleepTime
	s.mu.RUnlock()

	if sleepTime <= 0 {
		return defaultSleepTime
	}
	return sleepTime
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
		"blocked",
		"limited",
		"\u62e6\u622a",
		"\u9891\u7e41",
		"\u9650\u5236",
	} {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

func (s *Service) noteWindowMissing() {
	s.mu.Lock()
	s.windowMissStreak++
	s.windowStaticStreak = 0
	s.windowFingerprint = ""
	s.mu.Unlock()
}

func (s *Service) shouldSkipWindowDecode(fingerprint string, silent bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.windowMissStreak = 0
	if fingerprint == "" {
		s.windowStaticStreak = 0
		s.windowFingerprint = ""
		return false
	}

	if s.windowFingerprint != fingerprint {
		s.windowFingerprint = fingerprint
		s.windowStaticStreak = 0
		return false
	}

	s.windowStaticStreak++
	if !silent {
		return false
	}
	if s.windowStaticStreak < windowStaticSkipThreshold {
		return false
	}
	return s.windowStaticStreak%windowStaticDecodePeriod != 0
}

func windowImageFingerprint(img image.Image) string {
	if img == nil {
		return ""
	}
	bounds := img.Bounds()
	if bounds.Empty() || bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return ""
	}

	const grid = 8
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(fmt.Sprintf("%dx%d", bounds.Dx(), bounds.Dy())))
	for y := 0; y < grid; y++ {
		sampleY := bounds.Min.Y + ((2*y+1)*bounds.Dy())/(2*grid)
		for x := 0; x < grid; x++ {
			sampleX := bounds.Min.X + ((2*x+1)*bounds.Dx())/(2*grid)
			r, g, b, a := img.At(sampleX, sampleY).RGBA()
			luma := uint8((((r >> 8) * 299) + ((g >> 8) * 587) + ((b >> 8) * 114)) / 1000)
			_, _ = hasher.Write([]byte{luma, uint8(a >> 8)})
		}
	}
	return strconv.FormatUint(hasher.Sum64(), 16)
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
	return s.cfg.AccountLogin && s.sessionInfo != nil
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
		return localizedErrorf("backend.error.session_not_ready", nil, "game session is not ready; login first")
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

func (s *Service) scanWindowOnce(ctx context.Context, silent bool) (bool, MessageRef, error) {
	if err := s.prepareScanSession(ctx, silent); err != nil {
		return false, MessageRef{}, err
	}

	window, err := winwindow.FindFirst(targetWindowPattern, targetProcessNames...)
	if err != nil {
		if errors.Is(err, winwindow.ErrTargetWindowNotFound) {
			s.noteWindowMissing()
			s.mu.Lock()
			s.lastAction = "waiting_window"
			s.mu.Unlock()
			if !silent {
				s.emitState()
			}
			return false, MessageRef{}, nil
		}
		return false, MessageRef{}, err
	}
	if window.Bounds.Empty() || window.Bounds.Dx() <= 0 || window.Bounds.Dy() <= 0 {
		return false, MessageRef{}, fmt.Errorf("target window bounds are invalid")
	}

	img, err := winwindow.Capture(window)
	if err != nil {
		return false, MessageRef{}, err
	}
	fingerprint := windowImageFingerprint(img)
	if s.shouldSkipWindowDecode(fingerprint, silent) {
		hint, expandErr := s.tryExpandWindowQRCode(img)
		if expandErr != nil {
			s.logf("window QR expand attempt failed: %v", expandErr)
		}
		return false, hint, nil
	}

	ok, scanResult, consumeErr := s.consumeImageResult(ctx, img, false)
	if consumeErr == nil {
		if isExpiredScanResult(scanResult) {
			s.logf("scan reported expired QR; manual refresh is required")
			hint, hintErr := s.tryRefreshExpiredWindowQRCode(window, img)
			if hintErr != nil {
				s.logf("window QR state analysis failed: %v", hintErr)
			}
			return false, hint, nil
		}
		return ok, MessageRef{}, nil
	}
	hint, expandErr := s.tryExpandWindowQRCode(img)
	if expandErr != nil {
		s.logf("window QR state analysis failed after decode error: %v", expandErr)
	}
	if hint.Code != "" {
		return false, hint, nil
	}
	return false, MessageRef{}, nil
}

func (s *Service) consumeImage(ctx context.Context, img image.Image, clearClipboard bool) (bool, error) {
	ok, _, err := s.consumeImageResult(ctx, img, clearClipboard)
	return ok, err
}

func (s *Service) consumeImageResult(ctx context.Context, img image.Image, clearClipboard bool) (bool, ScanResult, error) {
	ticket, rawURL, err := qr.DecodeTicketFromImage(img)
	if err != nil {
		return false, ScanResult{}, err
	}
	return s.consumeTicketResult(ctx, ticket, rawURL, clearClipboard)
}

func (s *Service) consumeTicket(ctx context.Context, ticket, rawURL string, clearClipboard bool) (bool, error) {
	ok, _, err := s.consumeTicketResult(ctx, ticket, rawURL, clearClipboard)
	return ok, err
}

func (s *Service) consumeTicketResult(ctx context.Context, ticket, rawURL string, clearClipboard bool) (bool, ScanResult, error) {
	if !s.rememberTicket(ticket) {
		return false, ScanResult{}, nil
	}

	s.mu.Lock()
	s.lastTicket = ticket
	s.lastQRCodeURL = rawURL
	s.lastAction = "ticket_detected"
	s.lastError = ""
	s.lastErrorMessage = MessageRef{}
	s.mu.Unlock()

	scanResult, err := s.ScanTicket(ctx, ticket)
	if err != nil {
		return false, scanResult, err
	}

	if clearClipboard {
		clipboard.Write(clipboard.FmtImage, nil)
		clipboard.Write(clipboard.FmtText, nil)
	}

	s.emitState()
	return scanResult.OK, scanResult, nil
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

	if s.server != nil {
		s.server.ClearChallengeState()
	}

	s.mu.Lock()
	cfg := s.cfg.Clone()
	s.captchaPending = false
	s.captchaURL = ""
	s.lastAction = "login"
	s.lastError = ""
	s.lastErrorMessage = MessageRef{}
	s.quitRequested = false
	s.mu.Unlock()
	s.emitState()
	keepPendingCredentials := false
	defer func() {
		if !keepPendingCredentials {
			s.clearPendingCredentials()
		}
	}()

	result := LoginResult{}

	if s.bridge != nil {
		if account == "" {
			account = cfg.Account
		}
		if password == "" {
			password = cfg.Password
		}
		if account != "" && password != "" {
			s.storePendingCredentials(account, password)
		}

		var captchaReq *bridge.CaptchaPayload
		if cap != nil {
			captchaReq = &bridge.CaptchaPayload{
				Challenge: config.StringValue(cap["challenge"]),
				Validate:  config.StringValue(cap["validate"]),
				UserID:    config.StringValue(cap["userid"]),
			}
		}

		helperResp, err := s.bridge.Login(ctx, bridge.LoginRequest{
			Account:       account,
			Password:      password,
			UID:           cfg.UID,
			AccessKey:     cfg.AccessKey,
			LastLoginSucc: cfg.LastLoginSucc,
			Captcha:       captchaReq,
		})
		if err != nil {
			s.setError(err)
			return result, err
		}

		result.Message = helperResp.Message
		if helperResp.NeedsCaptcha {
			if err := s.startServer(); err != nil {
				s.setError(err)
				return result, err
			}
			if _, err := s.server.PrepareChallengeState(10 * time.Minute); err != nil {
				s.setError(err)
				return result, err
			}
			result.CaptchaURL = bsgamesdk.MakeCaptchaURL(
				s.server.Addr(),
				helperResp.CaptchaGT,
				helperResp.CaptchaChallenge,
				helperResp.CaptchaUserID,
			)
			result.NeedsCaptcha = result.CaptchaURL != ""

			s.mu.Lock()
			s.captchaPending = result.NeedsCaptcha
			s.captchaURL = result.CaptchaURL
			s.lastAction = "captcha_required"
			s.mu.Unlock()
			s.emitState()

			if result.NeedsCaptcha {
				keepPendingCredentials = true
				s.logf("captcha verification is required before login can continue")
				if openBrowser {
					_ = browser.OpenURL(result.CaptchaURL)
				}
				return result, nil
			}

			if result.Message == "" {
				result.MessageCode = "backend.error.bilibili_login_failed"
				result.Message = fallbackMessageText(newMessageRef(result.MessageCode, nil))
			}
			return result, nil
		}

		if helperResp.VerifyRetcode != 0 {
			err := localizedErrorf(
				"backend.error.verify_retcode",
				map[string]string{
					"source":  "Mihoyo",
					"retcode": strconv.FormatInt(helperResp.VerifyRetcode, 10),
				},
				"mihoyo verify retcode=%d",
				helperResp.VerifyRetcode,
			)
			s.setError(err)
			return result, err
		}

		if helperResp.AccessKey == "" {
			if result.Message == "" {
				result.MessageCode = "backend.error.bilibili_login_failed"
				result.Message = fallbackMessageText(newMessageRef(result.MessageCode, nil))
			}
			return result, nil
		}

		result.UName = strings.TrimSpace(helperResp.UName)
		if result.UName == "" {
			result.UName = strings.TrimSpace(account)
		}
		result.SessionReady = true

		s.mu.Lock()
		if account != "" {
			s.cfg.Account = account
		}
		if helperResp.UID != 0 {
			s.cfg.UID = helperResp.UID
		}
		s.cfg.AccessKey = helperResp.AccessKey
		s.cfg.LastLoginSucc = true
		if result.UName != "" {
			s.cfg.UName = result.UName
		}
		s.sessionInfo = cloneSessionInfo(&helperResp.Session)
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
		if dispatchResp != nil {
			result.DispatchReady = true
			result.DispatchSource = strings.TrimSpace(config.StringValue(dispatchResp["source"]))
		}

		s.mu.Lock()
		s.captchaPending = false
		s.captchaURL = ""
		saveErr = config.Save(s.cfgPath, s.cfg)
		s.mu.Unlock()
		if s.server != nil {
			s.server.ClearChallengeState()
		}
		if saveErr != nil {
			return result, saveErr
		}

		result.OK = true
		result.MessageCode = "common.ok"
		result.Message = "ok"
		s.logf("login completed")
		s.emitState()
		return result, nil
	}

	uid := fmt.Sprintf("%d", cfg.UID)
	accessKey := cfg.AccessKey
	var userInfo map[string]any

	if cfg.LastLoginSucc && cfg.UID != 0 && cfg.AccessKey != "" {
		info, err := s.bili.GetUserInfo(ctx, uid, accessKey)
		if err == nil && config.StringValue(info["uname"]) != "" {
			userInfo = info
			result.UName = config.StringValue(info["uname"])
			result.SessionReady = true
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
			return result, localizedErrorf("backend.error.credentials_required", nil, "account and password are required")
		}
		s.storePendingCredentials(account, password)

		loginResp, err := s.bili.Login(ctx, account, password, cap)
		if err != nil {
			s.setError(err)
			return result, err
		}

		accessKey = config.StringValue(loginResp["access_key"])
		if accessKey == "" {
			result.Message = config.StringValue(loginResp["message"])
			capData, capErr := s.bili.StartCaptcha(ctx)
			if capErr == nil {
				if err := s.startServer(); err != nil {
					s.setError(err)
					return result, err
				}
				if _, err := s.server.PrepareChallengeState(10 * time.Minute); err != nil {
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
				keepPendingCredentials = true
				s.logf("captcha verification is required before login can continue")
				if openBrowser {
					_ = browser.OpenURL(result.CaptchaURL)
				}
				return result, nil
			}

			if result.Message == "" {
				result.MessageCode = "backend.error.bilibili_login_failed"
				result.Message = fallbackMessageText(newMessageRef(result.MessageCode, nil))
			}
			return result, nil
		}

		uid = strings.TrimSpace(config.StringValue(loginResp["uid"]))
		if uid != "" {
			info, err := s.bili.GetUserInfo(ctx, uid, accessKey)
			if err != nil {
				s.setError(err)
				return result, err
			}
			userInfo = info
			result.UName = config.StringValue(info["uname"])
			if result.UName == "" {
				err = fmt.Errorf("bilibili user info missing uname")
				s.setError(err)
				return result, err
			}
		}
		if result.UName == "" {
			result.UName = strings.TrimSpace(config.StringValue(loginResp["uname"]))
		}
		if result.UName == "" {
			result.UName = strings.TrimSpace(account)
		}

		s.mu.Lock()
		s.cfg.Account = account
		if parsedUID := config.Int64Value(uid); parsedUID != 0 {
			s.cfg.UID = parsedUID
		}
		s.cfg.AccessKey = accessKey
		s.cfg.LastLoginSucc = true
		if result.UName != "" {
			s.cfg.UName = result.UName
		}
		saveErr := config.Save(s.cfgPath, s.cfg)
		s.mu.Unlock()
		if saveErr != nil {
			return result, saveErr
		}
		s.logf("bilibili login succeeded")
		result.SessionReady = true
	}

	if userInfo == nil && uid != "" {
		info, err := s.bili.GetUserInfo(ctx, uid, accessKey)
		if err != nil {
			return result, err
		}
		userInfo = info
		result.UName = config.StringValue(info["uname"])
		if result.UName == "" {
			err = fmt.Errorf("bilibili user info missing uname")
			s.setError(err)
			return result, err
		}
		result.SessionReady = true
	}
	if result.UName == "" {
		result.UName = strings.TrimSpace(cfg.UName)
	}
	if result.UName == "" {
		result.UName = strings.TrimSpace(account)
	}

	verifyResp, err := s.mihoyo.Verify(ctx, strings.TrimSpace(uid), accessKey)
	if err != nil {
		s.setError(err)
		return result, err
	}
	result.SessionReady = true
	if config.Int64Value(verifyResp["retcode"]) != 0 {
		err = localizedErrorf(
			"backend.error.verify_retcode",
			map[string]string{
				"source":  "Mihoyo",
				"retcode": strconv.FormatInt(config.Int64Value(verifyResp["retcode"]), 10),
			},
			"mihoyo verify retcode=%d",
			config.Int64Value(verifyResp["retcode"]),
		)
		s.setError(err)
		return result, err
	}

	session, err := mihoyosdk.ExtractSessionInfo(verifyResp)
	if err != nil {
		s.setError(err)
		return result, err
	}

	s.mu.Lock()
	s.sessionInfo = cloneSessionInfo(session)
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
	if dispatchResp != nil {
		result.DispatchReady = true
		result.DispatchSource = strings.TrimSpace(config.StringValue(dispatchResp["source"]))
	}

	s.mu.Lock()
	s.captchaPending = false
	s.captchaURL = ""
	saveErr = config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	if s.server != nil {
		s.server.ClearChallengeState()
	}
	if saveErr != nil {
		return result, saveErr
	}

	result.OK = true
	result.MessageCode = "common.ok"
	result.Message = "ok"
	s.logf("login completed")
	s.emitState()
	return result, nil
}

func (s *Service) handleCaptchaResult(payload map[string]any) {
	account, password, ok := s.pendingCredentials()
	if !ok {
		s.logf("captcha callback received but account credentials are missing")
		return
	}

	go func(account string, password []byte) {
		defer wipeBytes(password)
		if _, err := s.login(context.Background(), account, string(password), payload, false); err != nil {
			s.logf("captcha login continuation failed: %v", err)
		}
	}(account, password)
}

func (s *Service) setError(err error) {
	if err == nil {
		return
	}
	message := s.sanitizeMessage(err.Error())
	messageRef := messageRefFromError(err)
	s.mu.Lock()
	s.lastError = message
	s.lastErrorMessage = messageRef
	s.mu.Unlock()
	if s.fileLog != nil {
		s.fileLog.Writef("error: %s", message)
	}
	s.emitState()
}

func (s *Service) setHint(ref MessageRef, logText string) {
	if ref.Code == "" {
		return
	}

	text := strings.TrimSpace(translateMessageRef(ref))
	if text == "" {
		text = strings.TrimSpace(logText)
	}
	if text == "" {
		return
	}

	now := time.Now()
	s.mu.Lock()
	if s.lastNoticeCode == ref.Code && now.Sub(s.lastNoticeAt) < 4*time.Second {
		s.mu.Unlock()
		return
	}
	s.lastError = text
	s.lastErrorMessage = cloneMessageRef(ref)
	s.lastNoticeCode = ref.Code
	s.lastNoticeAt = now
	s.mu.Unlock()

	if s.fileLog != nil {
		s.fileLog.Writef("hint: %s", s.sanitizeMessage(text))
	}
	s.emitState()
}

func translateMessageRef(ref MessageRef) string {
	if ref.Code == "" {
		return ""
	}
	if text := fallbackMessageText(ref); text != "" {
		return text
	}
	return ""
}

func summarizeScanResult(resp map[string]any) ScanResult {
	retcode := config.Int64Value(resp["retcode"])
	result := ScanResult{
		OK:        retcode == 0,
		Confirmed: retcode == 0,
		Retcode:   retcode,
		Message:   strings.TrimSpace(config.StringValue(resp["message"])),
	}
	if result.Message == "" && retcode != 0 {
		result.Message = "Scan confirmation did not complete."
	}
	return result
}

func newScanWindowResult(matched bool, ref MessageRef) ScanWindowResult {
	result := ScanWindowResult{Matched: matched}
	if ref.Code == "" {
		return result
	}
	result.MessageRef = cloneMessageRef(ref)
	result.Message = strings.TrimSpace(translateMessageRef(ref))
	if result.Message == "" {
		result.Message = fallbackMessageText(ref)
	}
	return result
}

func isExpiredScanResult(result ScanResult) bool {
	return result.Retcode == -106
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
	hi3uid := s.cfg.HI3UID
	biliHitoken := s.cfg.BILIHITOKEN
	s.mu.RUnlock()

	for _, secret := range []string{password, accessKey, hi3uid, biliHitoken} {
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

func (s *Service) syncGameVersion() (string, bool, *bilihitoken.ReleaseInfo, error) {
	cfg := s.Config()

	remoteInfo, _ := s.fetchReleaseInfo(context.Background())
	resolvedDir, localVersion := resolveLocalGameVersion(cfg.GamePath)
	versionLogDetail := describeVersionSync(versionFromReleaseInfo(remoteInfo), localVersion)
	selectedVersion := pickLatestVersion(
		versionFromReleaseInfo(remoteInfo),
		localVersion,
		cfg.BHVer,
	)

	var (
		changed        bool
		versionChanged bool
		saveErr        error
	)

	s.mu.Lock()
	if resolvedDir != "" && s.cfg.GamePath != resolvedDir {
		s.cfg.GamePath = resolvedDir
		changed = true
	}
	if remoteInfo != nil && remoteInfo.Version != 0 && s.cfg.BiliPkgVer != remoteInfo.Version {
		s.cfg.BiliPkgVer = remoteInfo.Version
		changed = true
	}
	if selectedVersion != "" && s.cfg.BHVer != selectedVersion {
		s.cfg.BHVer = selectedVersion
		s.cfg.ClearDispatchSnapshot()
		changed = true
		versionChanged = true
	}
	if changed {
		saveErr = config.Save(s.cfgPath, s.cfg)
	}
	s.mu.Unlock()
	if saveErr != nil {
		return "", false, remoteInfo, saveErr
	}

	if versionChanged {
		s.mihoyo.ResetCache()
		s.logf("selected BH3 version %s (%s)", selectedVersion, versionLogDetail)
	}
	if changed {
		s.emitState()
	}
	return selectedVersion, versionChanged, remoteInfo, nil
}

func (s *Service) syncVersionAndDispatch(ctx context.Context, forceDispatch bool) (map[string]any, error) {
	version, versionChanged, remoteInfo, err := s.syncGameVersion()
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

	return s.refreshDispatchData(ctx, version, remoteInfo)
}

func (s *Service) refreshDispatchData(ctx context.Context, version string, remoteInfo *bilihitoken.ReleaseInfo) (map[string]any, error) {
	cfg := s.Config()
	_, localVersion := resolveLocalGameVersion(cfg.GamePath)
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
	if openID == "" && comboToken == "" && uid == "" {
		return nil, nil
	}

	if remoteInfo == nil {
		info, err := s.fetchReleaseInfo(ctx)
		if err != nil {
			s.logf("fetch release info failed: %v", err)
		} else {
			remoteInfo = info
		}
	}

	if remoteInfo != nil {
		versionLogDetail := describeVersionSync(remoteInfo.BHVer, localVersion)
		version = pickLatestVersion(version, remoteInfo.BHVer, cfg.BHVer)
		if strings.TrimSpace(cfg.BILIHITOKEN) == "" && remoteInfo.Version != 0 {
			tokenResp, err := s.fetchCredential(ctx)
			if err == nil && strings.TrimSpace(tokenResp.Token) != "" {
				s.mu.Lock()
				s.cfg.BILIHITOKEN = tokenResp.Token
				s.cfg.BiliPkgVer = tokenResp.Version
				s.cfg.BHVer = pickLatestVersion(s.cfg.BHVer, tokenResp.BHVer)
				_ = config.Save(s.cfgPath, s.cfg)
				s.mu.Unlock()
				cfg.BILIHITOKEN = tokenResp.Token
				cfg.BiliPkgVer = tokenResp.Version
				cfg.BHVer = pickLatestVersion(cfg.BHVer, tokenResp.BHVer)
				version = pickLatestVersion(version, cfg.BHVer)
				comboToken = tokenResp.Token
				s.logf("filled missing BILIHITOKEN via remote fetch; pkg_ver=%d %s", tokenResp.Version, versionLogDetail)
			} else {
				s.logf("failed to fill missing BILIHITOKEN: %v", err)
			}
		}

		if remoteInfo.Version != 0 && remoteInfo.Version != cfg.BiliPkgVer {
			tokenResp, err := s.fetchCredential(ctx)
			if err == nil && strings.TrimSpace(tokenResp.Token) != "" {
				s.mu.Lock()
				s.cfg.BILIHITOKEN = tokenResp.Token
				s.cfg.BiliPkgVer = tokenResp.Version
				s.cfg.BHVer = pickLatestVersion(s.cfg.BHVer, tokenResp.BHVer)
				_ = config.Save(s.cfgPath, s.cfg)
				s.mu.Unlock()
				cfg.BILIHITOKEN = tokenResp.Token
				cfg.BiliPkgVer = tokenResp.Version
				cfg.BHVer = pickLatestVersion(cfg.BHVer, tokenResp.BHVer)
				version = pickLatestVersion(version, cfg.BHVer)
				comboToken = tokenResp.Token
				s.logf("auto-updated BILIHITOKEN to pkg_ver=%d %s", tokenResp.Version, versionLogDetail)
			} else {
				s.logf("auto-update BILIHITOKEN failed: %v", err)
				s.mu.Lock()
				s.lastErrorMessage = newMessageRef("backend.error.auto_fetch_bilihitoken_failed", nil)
				s.lastError = fallbackMessageText(s.lastErrorMessage)
				s.mu.Unlock()
				s.emitState()
			}
		}
	}

	if strings.TrimSpace(version) == "" && cfg != nil {
		version = strings.TrimSpace(cfg.BHVer)
	}
	if version != "" {
		cfg.BHVer = version
	}

	s.mihoyo.ResetDispatchCache()

	resp, err := s.resolveDispatch(ctx, cfg, uid, version)
	if err != nil {
		return nil, err
	}
	if retcode := config.Int64Value(resp["retcode"]); retcode != 0 && resp["retcode"] != nil {
		return resp, localizedErrorf(
			"backend.error.dispatch_retcode",
			map[string]string{"retcode": strconv.FormatInt(retcode, 10)},
			"dispatch retcode=%d",
			retcode,
		)
	}

	dispatchData := config.StringValue(resp["data"])
	if dispatchData == "" {
		return resp, localizedErrorf("backend.error.dispatch_missing_data", nil, "dispatch response missing data")
	}
	if !mihoyosdk.LooksLikeFinalDispatch(dispatchData) {
		return resp, localizedErrorf("backend.error.dispatch_invalid_blob", nil, "dispatch response is not a usable final blob")
	}

	entry := buildDispatchCacheEntry(resp)

	s.mu.Lock()
	if version != "" {
		s.cfg.BHVer = version
	}
	s.cfg.SetDispatchSnapshot(version, entry)
	saveErr := config.Save(s.cfgPath, s.cfg)
	currentVersion := s.cfg.BHVer
	s.mu.Unlock()
	if saveErr != nil {
		return resp, saveErr
	}

	s.applyDispatchState(resp)

	source := config.StringValue(resp["source"])
	if source == "" {
		source = "unknown"
	}
	s.logf("dispatch refreshed for version %s via %s", currentVersion, source)
	s.emitState()
	return resp, nil
}

// ManualRefreshDispatch sets HI3UID and BILIHITOKEN in config and triggers a dispatch refresh.
func (s *Service) ManualRefreshDispatch(ctx context.Context, hi3uid, biliHitoken string) (State, error) {
	s.mu.Lock()
	if uid := strings.TrimSpace(hi3uid); uid != "" {
		s.cfg.HI3UID = uid
	}
	if token := strings.TrimSpace(biliHitoken); token != "" {
		s.cfg.BILIHITOKEN = token
	}
	if strings.TrimSpace(s.cfg.HI3UID) == "" || strings.TrimSpace(s.cfg.BILIHITOKEN) == "" {
		s.mu.Unlock()
		return s.State(), fmt.Errorf("HI3UID and BILIHITOKEN are required")
	}
	if err := config.Save(s.cfgPath, s.cfg); err != nil {
		s.mu.Unlock()
		return s.State(), err
	}
	s.mu.Unlock()

	version, _, remoteInfo, versionErr := s.syncGameVersion()
	if versionErr != nil {
		return s.State(), versionErr
	}
	_, err := s.refreshDispatchData(ctx, version, remoteInfo)
	s.emitState()
	return s.State(), err
}

// ManualFetchBiliHitoken attempts to refresh a local BILIHITOKEN
// through the configured private provider and saves it into config.
func (s *Service) ManualFetchBiliHitoken(ctx context.Context) (State, error) {
	cfg := s.Config()
	_, localVersion := resolveLocalGameVersion(cfg.GamePath)
	info, err := s.fetchCredential(ctx)
	if err != nil {
		return s.State(), err
	}
	if strings.TrimSpace(info.Token) == "" {
		return s.State(), localizedErrorf("backend.error.empty_bilihitoken", nil, "fetched empty BILIHITOKEN")
	}

	s.mu.Lock()
	s.cfg.BILIHITOKEN = info.Token
	s.cfg.BiliPkgVer = info.Version
	if info.BHVer != "" {
		s.cfg.BHVer = pickLatestVersion(s.cfg.BHVer, info.BHVer)
	}
	saveErr := config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	if saveErr != nil {
		return s.State(), saveErr
	}

	s.logf("manually fetched BILIHITOKEN pkg_ver=%d %s", info.Version, describeVersionSync(info.BHVer, localVersion))
	s.emitState()
	return s.State(), nil
}
func (s *Service) useConfiguredDispatch(version string) (map[string]any, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !mihoyosdk.LooksLikeFinalDispatch(s.cfg.DispatchData) {
		return nil, false, nil
	}
	officialMode := mihoyosdk.UsesPrivateDispatch(s.cfg.DispatchAPI)
	if officialMode {
		// In official mode, always prefer official request path over static dispatch_data.
		return nil, false, nil
	}

	key := config.NormalizeDispatchVersion(version)
	entry := buildDispatchCacheEntry(map[string]any{
		"data":   s.cfg.DispatchData,
		"source": "local_config",
	})
	changed := false
	if key != "" {
		changed = s.cfg.SetDispatchSnapshot(key, entry)
	}

	if changed {
		if err := config.Save(s.cfgPath, s.cfg); err != nil {
			return nil, false, err
		}
	}

	return dispatchResponseFromCache(key, entry, "local_config"), true, nil
}

func (s *Service) activateCachedDispatch(version string) (map[string]any, bool, error) {
	key := config.NormalizeDispatchVersion(version)
	if key == "" {
		return nil, false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snapshotVersion, entry, ok := s.cfg.DispatchSnapshot()
	if !ok || snapshotVersion != key || !mihoyosdk.LooksLikeFinalDispatch(entry.Data) {
		return nil, false, nil
	}
	officialMode := mihoyosdk.UsesPrivateDispatch(s.cfg.DispatchAPI)
	if officialMode && mihoyosdk.ShouldSkipPreferredDispatchCacheSource(entry.Source) {
		return nil, false, nil
	}

	changed := false
	changed = s.cfg.SetDispatchSnapshot(key, entry)
	if changed {
		if err := config.Save(s.cfgPath, s.cfg); err != nil {
			return nil, false, err
		}
	}

	return dispatchResponseFromCache(key, entry, "local_cache"), true, nil
}

func dispatchResponseFromCache(key string, entry config.DispatchCacheEntry, source string) map[string]any {
	resp := map[string]any{
		"retcode": 0,
		"message": "OK",
		"data":    entry.Data,
		"source":  source,
	}
	if key != "" {
		resp["cache_key"] = key
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

func (s *Service) fetchReleaseInfo(ctx context.Context) (*bilihitoken.ReleaseInfo, error) {
	if s.bridge != nil {
		info, err := s.bridge.FetchReleaseInfo(ctx)
		if err != nil {
			return nil, err
		}
		return &bilihitoken.ReleaseInfo{
			Version: info.Version,
			BHVer:   strings.TrimSpace(info.BHVer),
		}, nil
	}
	return bilihitoken.FetchReleaseInfo(netutil.NewClient())
}

func (s *Service) fetchCredential(ctx context.Context) (bridge.CredentialResponse, error) {
	if s.bridge != nil {
		return s.bridge.FetchCredential(ctx)
	}
	httpClient := netutil.NewClient()
	info, err := bilihitoken.FetchReleaseInfo(httpClient)
	if err != nil {
		return bridge.CredentialResponse{}, err
	}
	token, err := bilihitoken.FetchCredential(httpClient, info.PackageURL)
	if err != nil {
		return bridge.CredentialResponse{}, err
	}
	return bridge.CredentialResponse{
		Token:   strings.TrimSpace(token),
		Version: info.Version,
		BHVer:   strings.TrimSpace(info.BHVer),
	}, nil
}

func (s *Service) resolveDispatch(ctx context.Context, cfg *config.Config, uid, version string) (map[string]any, error) {
	if s.bridge != nil {
		resp, err := s.bridge.ResolveDispatch(ctx, bridge.DispatchRequest{
			Config:  bridgeConfigSnapshot(cfg),
			UID:     uid,
			Version: version,
		})
		if err != nil {
			return nil, err
		}
		return dispatchResponseFromBridge(resp), nil
	}
	return s.mihoyo.GetOAServer(ctx, uid, cfg)
}

func bridgeConfigSnapshot(cfg *config.Config) bridge.ConfigSnapshot {
	if cfg == nil {
		cfg = config.Default()
	}
	return bridge.ConfigSnapshot{
		GamePath:              cfg.GamePath,
		BHVer:                 cfg.BHVer,
		BiliPkgVer:            cfg.BiliPkgVer,
		VersionAPI:            cfg.VersionAPI,
		DispatchAPI:           cfg.DispatchAPI,
		DispatchData:          cfg.DispatchData,
		DispatchVersion:       cfg.DispatchVersion,
		DispatchSource:        cfg.DispatchSource,
		DispatchRawLen:        cfg.DispatchRawLen,
		DispatchDecodedLen:    cfg.DispatchDecodedLen,
		DispatchDecodedSHA256: cfg.DispatchDecodedSHA256,
		DispatchSavedAt:       cfg.DispatchSavedAt,
		BILIHITOKEN:           cfg.BILIHITOKEN,
		HI3UID:                cfg.HI3UID,
	}
}

func dispatchResponseFromBridge(resp bridge.DispatchResponse) map[string]any {
	out := map[string]any{
		"retcode": resp.Retcode,
		"message": resp.Message,
		"data":    resp.Data,
		"source":  resp.Source,
	}
	if resp.CachedSource != "" {
		out["cached_source"] = resp.CachedSource
	}
	if resp.CachedSavedAt != "" {
		out["cached_saved_at"] = resp.CachedSavedAt
	}
	if resp.BlobSummary.RawLen > 0 || resp.BlobSummary.DecodedLen > 0 || resp.BlobSummary.DecodedSHA256 != "" {
		out["blob_summary"] = map[string]any{
			"raw_len":        resp.BlobSummary.RawLen,
			"decoded_len":    resp.BlobSummary.DecodedLen,
			"decoded_sha256": resp.BlobSummary.DecodedSHA256,
		}
	}
	return out
}

func (s *Service) currentOpenID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.sessionInfo == nil {
		return ""
	}
	return s.sessionInfo.OpenID
}

func (s *Service) currentComboToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.sessionInfo == nil {
		return ""
	}
	return s.sessionInfo.ComboToken
}

func (s *Service) currentUID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.sessionInfo != nil && s.sessionInfo.UID != "" {
		return s.sessionInfo.UID
	}
	if s.cfg != nil && s.cfg.UID != 0 {
		return fmt.Sprintf("%d", s.cfg.UID)
	}
	return ""
}

func cloneSessionInfo(src *mihoyosdk.SessionInfo) *mihoyosdk.SessionInfo {
	if src == nil {
		return nil
	}
	clone := *src
	return &clone
}

func sanitizeResultMessage(result map[string]any) string {
	if result == nil {
		return "unknown"
	}
	message := strings.TrimSpace(config.StringValue(result["message"]))
	if message == "" {
		return "unknown"
	}
	return strings.ReplaceAll(sanitizeMessageStatic(message), "\n", " ")
}

func sanitizeMessageStatic(message string) string {
	for _, rule := range sensitiveLogRules {
		message = rule.pattern.ReplaceAllString(message, rule.replacement)
	}
	return message
}

func wipeBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

func (s *Service) storePendingCredentials(account, password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingPasswordGen++
	s.clearPendingCredentialsLocked()
	s.pendingAccount = strings.TrimSpace(account)
	s.pendingPassword = append([]byte(nil), []byte(password)...)
	s.pendingPasswordTTL = time.Now().Add(pendingCredentialTTL)
	gen := s.pendingPasswordGen
	s.pendingPasswordTmr = time.AfterFunc(pendingCredentialTTL, func() {
		s.expirePendingCredentials(gen)
	})
}

func (s *Service) clearPendingCredentials() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingPasswordGen++
	s.clearPendingCredentialsLocked()
}

func (s *Service) clearPendingCredentialsLocked() {
	if s.pendingPasswordTmr != nil {
		s.pendingPasswordTmr.Stop()
		s.pendingPasswordTmr = nil
	}
	if len(s.pendingPassword) > 0 {
		wipeBytes(s.pendingPassword)
	}
	s.pendingPassword = nil
	s.pendingAccount = ""
	s.pendingPasswordTTL = time.Time{}
}

func (s *Service) pendingCredentials() (string, []byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(s.pendingAccount) == "" || len(s.pendingPassword) == 0 {
		return "", nil, false
	}
	if !s.pendingPasswordTTL.IsZero() && time.Now().After(s.pendingPasswordTTL) {
		s.clearPendingCredentialsLocked()
		return "", nil, false
	}
	password := append([]byte(nil), s.pendingPassword...)
	return s.pendingAccount, password, true
}

func (s *Service) expirePendingCredentials(generation uint64) {
	clearChallengeState := false
	shouldEmit := false

	s.mu.Lock()
	if generation != s.pendingPasswordGen || strings.TrimSpace(s.pendingAccount) == "" || len(s.pendingPassword) == 0 {
		s.mu.Unlock()
		return
	}
	s.clearPendingCredentialsLocked()
	if s.captchaPending || s.captchaURL != "" {
		s.captchaPending = false
		s.captchaURL = ""
		if s.lastAction == "captcha_required" {
			s.lastAction = "captcha_expired"
		}
		clearChallengeState = true
		shouldEmit = true
	}
	s.mu.Unlock()

	if clearChallengeState && s.server != nil {
		s.server.ClearChallengeState()
	}
	s.logf("captcha login timed out; pending credentials cleared")
	if shouldEmit {
		s.emitState()
	}
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

func managedBackgroundPathFor() (string, error) {
	// Store managed background in the executable directory so packaged apps carry it alongside the binary.
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, managedBackgroundBaseName+managedBackgroundExt), nil
}

func backgroundContentType(path string, data []byte) string {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(path)))
	if ext == managedBackgroundExt {
		return "image/webp"
	}
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	return contentType
}

func encodeManagedBackground(source []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(source))
	if err != nil {
		return nil, fmt.Errorf("decode background image: %w", err)
	}

	var buf bytes.Buffer
	if err := purewebp.Encode(&buf, img, &purewebp.EncoderOptions{
		Quality: 80,
		Method:  4,
		Preset:  purewebp.PresetPhoto,
	}); err != nil {
		return nil, fmt.Errorf("encode background image: %w", err)
	}
	return buf.Bytes(), nil
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
	contentType := backgroundContentType(path, data)
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
	contentType := backgroundContentType(sourcePath, data)
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return "", "", fmt.Errorf("selected file is not an image")
	}

	encodedData, err := encodeManagedBackground(data)
	if err != nil {
		return "", "", err
	}

	destAbsPath, err := managedBackgroundPathFor()
	if err != nil {
		return "", "", err
	}
	if err := config.AtomicWriteFile(destAbsPath, encodedData, 0o644); err != nil {
		return "", "", fmt.Errorf("write background image: %w", err)
	}

	if previousPath != "" && !samePath(previousPath, destAbsPath) &&
		strings.HasPrefix(strings.ToLower(filepath.Base(previousPath)), managedBackgroundBaseName+".") {
		_ = os.Remove(previousPath)
	}

	sum := sha256.Sum256(encodedData)
	s.logf(
		"background image imported as %s (%s, %d -> %d bytes)",
		destAbsPath,
		hex.EncodeToString(sum[:8]),
		len(data),
		len(encodedData),
	)

	// Store relative path in config for portability (relative to executable dir)
	rel := "./" + filepath.Base(destAbsPath)
	return rel, "data:image/webp;base64," + base64.StdEncoding.EncodeToString(encodedData), nil
}

func resolveLocalGameVersion(gamePath string) (string, string) {
	gamePath = strings.TrimSpace(gamePath)
	if gamePath == "" {
		return "", ""
	}

	dir, err := gameclient.ResolveDir(gamePath)
	if err != nil {
		return "", ""
	}

	version, err := gameclient.ReadVersion(dir)
	if err != nil {
		return dir, ""
	}
	return dir, strings.TrimSpace(version)
}

func versionFromReleaseInfo(info *bilihitoken.ReleaseInfo) string {
	if info == nil {
		return ""
	}
	return strings.TrimSpace(info.BHVer)
}

func pickLatestVersion(candidates ...string) string {
	best := ""
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if best == "" || compareVersionStrings(candidate, best) > 0 {
			best = candidate
		}
	}
	return best
}

func compareVersionStrings(left, right string) int {
	leftParts, leftOK := parseVersionParts(left)
	rightParts, rightOK := parseVersionParts(right)

	switch {
	case leftOK && !rightOK:
		return 1
	case !leftOK && rightOK:
		return -1
	case !leftOK && !rightOK:
		return 0
	}

	maxLen := len(leftParts)
	if len(rightParts) > maxLen {
		maxLen = len(rightParts)
	}
	for i := 0; i < maxLen; i++ {
		leftValue := 0
		if i < len(leftParts) {
			leftValue = leftParts[i]
		}
		rightValue := 0
		if i < len(rightParts) {
			rightValue = rightParts[i]
		}
		switch {
		case leftValue > rightValue:
			return 1
		case leftValue < rightValue:
			return -1
		}
	}
	return 0
}

func parseVersionParts(version string) ([]int, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, false
	}

	match := versionStringPattern.FindString(version)
	if match == "" {
		return nil, false
	}

	rawParts := strings.Split(match, ".")
	parts := make([]int, 0, len(rawParts))
	for _, rawPart := range rawParts {
		value, err := strconv.Atoi(rawPart)
		if err != nil {
			return nil, false
		}
		parts = append(parts, value)
	}
	return parts, len(parts) > 0
}

func describeVersionSync(remoteVersion, localVersion string) string {
	remoteVersion = strings.TrimSpace(remoteVersion)
	localVersion = strings.TrimSpace(localVersion)

	switch {
	case remoteVersion == "" && localVersion == "":
		return "remote_bhver=unknown local_bhver=unknown version_check=unavailable"
	case remoteVersion == "":
		return fmt.Sprintf("remote_bhver=unknown local_bhver=%s version_check=remote_bhver_unavailable", localVersion)
	case localVersion == "":
		return fmt.Sprintf("remote_bhver=%s local_bhver=unknown version_check=local_bhver_unavailable", remoteVersion)
	}

	switch compareVersionStrings(remoteVersion, localVersion) {
	case 1:
		return fmt.Sprintf("remote_bhver=%s local_bhver=%s version_check=local_game_needs_update", remoteVersion, localVersion)
	case -1:
		return fmt.Sprintf("remote_bhver=%s local_bhver=%s version_check=remote_source_needs_update", remoteVersion, localVersion)
	default:
		return fmt.Sprintf("remote_bhver=%s local_bhver=%s version_check=in_sync", remoteVersion, localVersion)
	}
}

func evaluateGamePath(path string) (bool, MessageRef) {
	path = strings.TrimSpace(path)
	if path == "" {
		return false, newMessageRef("backend.hint.game_path_missing", nil)
	}
	if _, err := gameclient.ResolveDir(path); err != nil {
		return false, newMessageRef("backend.hint.game_path_invalid", nil)
	}
	return true, MessageRef{}
}
