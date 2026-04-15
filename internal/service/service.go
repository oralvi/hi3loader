package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"hi3loader/internal/bridge"
	"hi3loader/internal/bsgamesdk"
	"hi3loader/internal/captcha"
	"hi3loader/internal/config"
	"hi3loader/internal/debuglog"
	"hi3loader/internal/gameclient"
	"hi3loader/internal/qr"
	"hi3loader/internal/winwindow"

	"github.com/pkg/browser"
)

const (
	defaultConfigPath         = "config.json"
	maxLogEntries             = 300
	ticketTTL                 = 30 * time.Second
	pendingCredentialTTL      = 10 * time.Minute
	apiProbeInterval          = 5 * time.Second
	apiProbeTimeout           = 6 * time.Second
	defaultSleepTime          = 3
	windowStaticSkipThreshold = 2
	windowStaticDecodePeriod  = 3
	managedBackgroundBaseName = "custom_background"
	hintStateInterval         = 6 * time.Second
	hintLogInterval           = 12 * time.Second
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
		{regexp.MustCompile(`(?i)(password=)([^&\s]+)`), `${1}***`},
		{regexp.MustCompile(`(?i)(access_key=)([^&\s]+)`), `${1}***`},
	}
)

type LogEntry struct {
	At      string `json:"at"`
	Message string `json:"message"`
}

type State struct {
	Config           ConfigView    `json:"config"`
	BuildInfo        BuildInfoView `json:"buildInfo"`
	Running          bool          `json:"running"`
	RuntimePreparing bool          `json:"runtimePreparing"`
	APIReady         bool          `json:"apiReady"`
	ServerAddress    string        `json:"serverAddress"`
	ServerReady      bool          `json:"serverReady"`
	GamePathValid    bool          `json:"gamePathValid"`
	GamePathPrompt   string        `json:"gamePathPrompt"`
	GamePathMessage  MessageRef    `json:"gamePathMessage"`
	GamePathNote     string        `json:"gamePathNote"`
	LauncherPathNote string        `json:"launcherPathNote"`
	LogPath          string        `json:"logPath"`
	CaptchaURL       string        `json:"captchaURL"`
	CaptchaPending   bool          `json:"captchaPending"`
	LastAction       string        `json:"lastAction"`
	LastError        string        `json:"lastError"`
	LastErrorMessage MessageRef    `json:"lastErrorMessage"`
	LastTicket       string        `json:"lastTicket"`
	LastQRCodeURL    string        `json:"lastQRCodeURL"`
	QuitRequested    bool          `json:"quitRequested"`
}

type LoginResult struct {
	OK            bool              `json:"ok"`
	NeedsCaptcha  bool              `json:"needsCaptcha"`
	CaptchaURL    string            `json:"captchaURL,omitempty"`
	Message       string            `json:"message,omitempty"`
	MessageCode   string            `json:"messageCode,omitempty"`
	MessageParams map[string]string `json:"messageParams,omitempty"`
	UName         string            `json:"uname,omitempty"`
	SessionReady  bool              `json:"sessionReady,omitempty"`
	Retcode       int64             `json:"retcode,omitempty"`
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

	bili *bsgamesdk.Client

	mu                  sync.RWMutex
	cfg                 *config.Config
	server              *captcha.Server
	serverReady         bool
	serverStarted       bool
	running             bool
	runtimePreparing    bool
	apiReady            bool
	loopCancel          context.CancelFunc
	monitorWake         chan struct{}
	captchaURL          string
	captchaPending      bool
	lastAction          string
	lastError           string
	lastErrorMessage    MessageRef
	lastNoticeCode      string
	lastNoticeAt        time.Time
	lastNoticeLogAt     time.Time
	lastTicket          string
	lastQRCodeURL       string
	quitRequested       bool
	logs                []LogEntry
	backgroundDataURL   string
	recentTickets       map[string]time.Time
	windowMissStreak    int
	windowStaticStreak  int
	windowFingerprint   string
	monitorPauseDepth   int
	apiInteractionDepth int
	gamePathNote        string
	launcherPathNote    string
	hooks               Hooks
	fileLog             *debuglog.Logger
	loginMu             sync.Mutex
	pendingAccount      string
	pendingPassword     []byte
	pendingRememberPass bool
	pendingPasswordTTL  time.Time
	pendingPasswordGen  uint64
	pendingPasswordTmr  *time.Timer
	module              *moduleRuntime
}

func New(cfgPath string) (*Service, error) {
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

	s := &Service{
		cfgPath:       cfgPath,
		cfg:           cfg,
		bili:          bsgamesdk.NewClient(),
		fileLog:       fileLog,
		monitorWake:   make(chan struct{}, 1),
		recentTickets: map[string]time.Time{},
		module:        newModuleRuntime(),
	}
	s.server = captcha.NewServer("127.0.0.1:0", s.handleCaptchaResult)
	s.autoPopulateRuntimePaths()
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
	if pending, err := s.ensureBundledModule(ctx); err != nil {
		return s.State(), err
	} else if pending {
		return s.State(), nil
	}

	if err := s.requireLoaderAPIConfigured(); err != nil {
		s.setError(err)
		return s.State(), err
	}
	if err := s.refreshAPIHealth(ctx); err != nil {
		s.logf("loader api probe failed: %v", err)
	}

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

	go s.monitorLoop(ctx)
	go s.apiHealthLoop(ctx)
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
	if s.module != nil {
		s.module.stop()
	}
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
	apiReady := s.apiReady
	runtimePreparing := s.runtimePreparing
	serverReady := s.serverReady
	logPath := s.logPath()
	captchaURL := s.captchaURL
	captchaPending := s.captchaPending
	gamePathNote := s.gamePathNote
	launcherPathNote := s.launcherPathNote
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
		BuildInfo:        currentBuildInfo(),
		Running:          running,
		RuntimePreparing: runtimePreparing,
		APIReady:         apiReady,
		ServerAddress:    s.server.Addr(),
		ServerReady:      serverReady,
		GamePathValid:    gamePathValid,
		GamePathPrompt:   gamePathPrompt,
		GamePathMessage:  gamePathMessage,
		GamePathNote:     gamePathNote,
		LauncherPathNote: launcherPathNote,
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

func (s *Service) SaveFeatureSettings(gamePath string, autoClose, autoWindowCapture, panelBlur bool, opacity float64) (State, error) {
	resolvedGamePath := ""
	if strings.TrimSpace(gamePath) != "" {
		dir, err := gameclient.ResolveDir(gamePath)
		if err != nil {
			return State{}, err
		}
		resolvedGamePath = dir
	}
	opacity = clampBackgroundOpacity(opacity)

	s.mu.Lock()
	if s.cfg == nil {
		s.mu.Unlock()
		return s.State(), fmt.Errorf("config is not loaded")
	}
	prevAutoWindowCapture := s.cfg.AutoWindowCapture

	nextCfg := s.cfg.Clone()
	nextCfg.GamePath = resolvedGamePath
	nextCfg.AutoClose = autoClose
	nextCfg.AutoWindowCapture = autoWindowCapture
	nextCfg.PanelBlur = panelBlur
	nextCfg.BackgroundOpacity = opacity
	nextCfg.SleepTime = defaultSleepTime
	err := config.Save(s.cfgPath, nextCfg)
	if err == nil {
		s.cfg = nextCfg
		s.gamePathNote = ""
	}
	s.mu.Unlock()
	if err != nil {
		return State{}, err
	}
	s.emitState()
	if autoWindowCapture || prevAutoWindowCapture {
		s.wakeMonitorLoop()
	}
	return s.State(), nil
}

func (s *Service) SaveLauncherPath(launcherPath string) (State, error) {
	resolvedLauncherPath := ""
	if strings.TrimSpace(launcherPath) != "" {
		exePath, err := gameclient.ResolveLauncherExecutable(launcherPath)
		if err != nil {
			return State{}, err
		}
		resolvedLauncherPath = exePath
	}

	s.mu.Lock()
	if s.cfg == nil {
		s.mu.Unlock()
		return s.State(), fmt.Errorf("config is not loaded")
	}

	nextCfg := s.cfg.Clone()
	nextCfg.LauncherPath = resolvedLauncherPath
	err := config.Save(s.cfgPath, nextCfg)
	if err == nil {
		s.cfg = nextCfg
		s.launcherPathNote = ""
	}
	s.mu.Unlock()
	if err != nil {
		return State{}, err
	}

	s.emitState()
	return s.State(), nil
}

func (s *Service) SaveCredentialSettings(asteriskName, loaderAPIBaseURL string) (State, error) {
	loaderAPIBaseURL = strings.TrimSpace(loaderAPIBaseURL)
	s.mu.Lock()
	if s.cfg == nil {
		s.mu.Unlock()
		return s.State(), fmt.Errorf("config is not loaded")
	}
	nextCfg := s.cfg.Clone()
	nextCfg.AsteriskName = strings.TrimSpace(asteriskName)
	nextCfg.LoaderAPIBaseURL = loaderAPIBaseURL
	err := config.Save(s.cfgPath, nextCfg)
	if err == nil {
		s.cfg = nextCfg
		s.apiReady = false
		s.lastError = ""
		s.lastErrorMessage = MessageRef{}
	}
	s.mu.Unlock()
	if err != nil {
		return s.State(), err
	}
	if loaderAPIBaseURL != "" {
		if err := s.refreshAPIHealth(context.Background()); err != nil {
			s.logf("loader api probe failed after settings update: %v", err)
		}
	}

	s.emitState()
	return s.State(), nil
}

func (s *Service) RecordClientMessage(message string) {
	message = strings.TrimSpace(s.sanitizeMessage(message))
	if message == "" || s.fileLog == nil {
		return
	}
	s.fileLog.Writef("%s", message)
}

func (s *Service) Note(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	s.logf("%s", message)
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

func (s *Service) CancelCaptchaLogin() State {
	clearChallengeState := false
	shouldLog := false

	s.mu.Lock()
	if strings.TrimSpace(s.pendingAccount) != "" || len(s.pendingPassword) > 0 || s.captchaPending || s.captchaURL != "" {
		s.pendingPasswordGen++
		s.clearPendingCredentialsLocked()
		if s.captchaPending || s.captchaURL != "" {
			clearChallengeState = true
		}
		s.captchaPending = false
		s.captchaURL = ""
		if s.lastAction == "captcha_required" {
			s.lastAction = "waiting_login"
		}
		s.lastError = ""
		s.lastErrorMessage = MessageRef{}
		shouldLog = true
	}
	s.mu.Unlock()

	if clearChallengeState && s.server != nil {
		s.server.ClearChallengeState()
	}
	if shouldLog {
		s.logf("captcha login flow cancelled")
	}
	s.emitState()
	return s.State()
}

func (s *Service) ReloadCaptchaLogin(ctx context.Context) (LoginResult, error) {
	account, password, rememberPassword, ok := s.pendingCredentials()
	if !ok {
		return LoginResult{}, localizedErrorf("backend.error.credentials_required", nil, "account and password are required")
	}
	defer wipeBytes(password)

	if err := s.prepareBSGameSDK(ctx, account); err != nil {
		return LoginResult{}, err
	}
	return s.login(ctx, account, string(password), rememberPassword, nil, false)
}

func (s *Service) Login(ctx context.Context, account, password string, rememberPassword, openBrowser bool) (LoginResult, error) {
	return s.login(ctx, account, password, rememberPassword, nil, openBrowser)
}

func (s *Service) SelectSavedAccount(account string) (State, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return s.State(), fmt.Errorf("saved account is required")
	}

	s.mu.Lock()
	if s.cfg == nil {
		s.mu.Unlock()
		return s.State(), fmt.Errorf("config is not loaded")
	}
	nextCfg := s.cfg.Clone()
	if !nextCfg.ApplySavedAccount(account) {
		s.mu.Unlock()
		return s.State(), fmt.Errorf("saved account not found")
	}
	if err := config.Save(s.cfgPath, nextCfg); err != nil {
		s.mu.Unlock()
		return s.State(), err
	}
	s.cfg = nextCfg
	s.mu.Unlock()
	s.clearPendingCredentials()
	s.restartMonitorContext()

	s.logf("switched active account to %s", maskSecret(account))
	s.emitState()
	return s.State(), nil
}

func (s *Service) ClearCurrentAccount() (State, error) {
	s.mu.Lock()
	if s.cfg == nil {
		s.mu.Unlock()
		return s.State(), fmt.Errorf("config is not loaded")
	}
	current, ok := s.cfg.CurrentSavedAccount()
	if !ok || strings.TrimSpace(current.Account) == "" {
		s.mu.Unlock()
		return s.State(), nil
	}

	nextCfg := s.cfg.Clone()
	if !nextCfg.RemoveSavedAccount(current.Account) {
		s.mu.Unlock()
		return s.State(), nil
	}
	if err := config.Save(s.cfgPath, nextCfg); err != nil {
		s.mu.Unlock()
		return s.State(), err
	}
	s.cfg = nextCfg
	s.mu.Unlock()

	s.clearPendingCredentials()
	s.restartMonitorContext()
	s.logf("cleared current account %s", maskSecret(current.Account))
	s.emitState()
	return s.State(), nil
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

func (s *Service) EnsureSession(ctx context.Context) error {
	s.mu.RLock()
	cfg := s.cfg.Clone()
	s.mu.RUnlock()
	active, _ := cfg.CurrentSavedAccount()
	ready := cfg.AccountLogin && strings.TrimSpace(active.AccessKey) != ""
	if ready {
		return nil
	}
	if strings.TrimSpace(active.AccessKey) == "" {
		return fmt.Errorf("missing cached access_key")
	}
	if err := s.prepareBSGameSDK(ctx, strings.TrimSpace(active.Account)); err != nil {
		return err
	}

	if active.UID != 0 {
		verifyUID := fmt.Sprintf("%d", active.UID)
		if _, err := s.bili.GetUserInfo(ctx, verifyUID, active.AccessKey); err != nil {
			s.clearCachedSession("cached session verify failed; login required again")
			return err
		}
	}

	s.mu.Lock()
	s.cfg.AccountLogin = true
	err := config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	if err != nil {
		return err
	}

	s.emitState()
	return nil
}

func (s *Service) clearCachedSession(reason string) {
	s.mu.Lock()
	currentAccount := s.cfg.CurrentAccount
	s.cfg.ClearSavedAccountSession(currentAccount)
	s.cfg.AccountLogin = false
	saveErr := config.Save(s.cfgPath, s.cfg)
	s.mu.Unlock()
	if saveErr != nil {
		s.logf("clear cached session failed: %v", saveErr)
		return
	}
	if strings.TrimSpace(reason) != "" {
		s.logf("%s", reason)
	}
	s.restartMonitorContext()
	s.emitState()
}

func (s *Service) saveAccountStateLocked(account, password string, rememberPassword bool, uid int64, accessKey, uname string, lastLoginSucc, accountLogin bool) error {
	entry, ok := s.cfg.CurrentSavedAccount()
	account = strings.TrimSpace(account)
	if account != "" {
		if existing, exists := s.cfg.FindSavedAccount(account); exists {
			entry = existing
			ok = true
		} else {
			entry = config.SavedAccount{Account: account}
			ok = true
		}
	}
	if !ok || strings.TrimSpace(entry.Account) == "" {
		return fmt.Errorf("account is required")
	}
	if account == "" {
		account = entry.Account
	}
	entry.Account = account
	entry.RememberPassword = rememberPassword
	if password != "" {
		entry.Password = password
	}
	if uid != 0 {
		entry.UID = uid
	}
	entry.AccessKey = accessKey
	entry.LastLoginSucc = lastLoginSucc
	if uname != "" {
		entry.UName = uname
	}
	if strings.TrimSpace(entry.AccessKey) == "" {
		entry.LastLoginSucc = false
	}
	s.cfg.CurrentAccount = entry.Account
	s.cfg.UpsertSavedAccount(entry)
	s.cfg.AccountLogin = accountLogin
	return config.Save(s.cfgPath, s.cfg)
}

func (s *Service) ScanTicket(ctx context.Context, ticket string) (ScanResult, error) {
	if ticket == "" {
		return ScanResult{}, localizedErrorf("backend.error.ticket_required", nil, "ticket is required")
	}
	if err := s.EnsureSession(ctx); err != nil {
		return ScanResult{}, err
	}

	s.mu.RLock()
	cfg := s.cfg.Clone()
	s.mu.RUnlock()
	active, _ := cfg.CurrentSavedAccount()

	s.beginAPIInteraction()
	defer s.endAPIInteraction()
	scanResp, scanErr := bridge.ExecuteScan(ctx, bridge.ScanRequest{
		Ticket:       ticket,
		UID:          active.UID,
		AccessKey:    active.AccessKey,
		AsteriskName: cfg.AsteriskName,
		LoaderAPIURL: cfg.LoaderAPIBaseURL,
		ClientMeta:   bridge.ClientMetaForBaseURL(cfg.LoaderAPIBaseURL),
	})
	if scanErr != nil {
		s.setAPIReady(false)
		s.setError(scanErr)
		return ScanResult{}, scanErr
	}
	s.setAPIReady(true)
	result := map[string]any{
		"retcode": scanResp.Retcode,
		"message": scanResp.Message,
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

func (s *Service) ScanWindow(ctx context.Context) (ScanWindowResult, error) {
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
	return ScanWindowResult{Matched: false}, nil
}

func (s *Service) monitorLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		cfg := s.Config()
		paused := s.monitorPaused()
		if !paused && cfg.AutoWindowCapture {
			if _, _, err := s.scanWindowOnce(ctx, true); err != nil {
				s.logf("window capture failed: %v", err)
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

func (s *Service) apiHealthLoop(ctx context.Context) {
	ticker := time.NewTicker(apiProbeInterval)
	defer ticker.Stop()

	s.pollAPIHealthOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollAPIHealthOnce(ctx)
		}
	}
}

func (s *Service) pollAPIHealthOnce(parent context.Context) {
	cfg := s.Config()
	if !s.usesLoaderAPIConfig(cfg) {
		s.setAPIReady(false)
		return
	}
	if s.isAPIInteractionActive() {
		return
	}

	s.mu.RLock()
	wasReady := s.apiReady
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(parent, apiProbeTimeout)
	defer cancel()

	err := bridge.ProbeLoaderAPI(ctx, cfg.LoaderAPIBaseURL, bridge.ClientMetaForBaseURL(cfg.LoaderAPIBaseURL))
	s.setAPIReady(err == nil)

	if err != nil {
		if wasReady {
			s.logf("loader api probe failed: %v", err)
		}
		return
	}
	if !wasReady {
		s.logf("loader api probe recovered: %s", strings.TrimSpace(cfg.LoaderAPIBaseURL))
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

func (s *Service) PauseMonitor() {
	s.mu.Lock()
	s.monitorPauseDepth++
	s.mu.Unlock()
}

func (s *Service) ResumeMonitor() {
	shouldWake := false
	s.mu.Lock()
	if s.monitorPauseDepth > 0 {
		s.monitorPauseDepth--
	}
	shouldWake = s.monitorPauseDepth == 0
	s.mu.Unlock()
	if shouldWake {
		s.wakeMonitorLoop()
	}
}

func (s *Service) monitorPaused() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.monitorPauseDepth > 0
}

func (s *Service) resetMonitorStateLocked() {
	s.recentTickets = map[string]time.Time{}
	s.windowMissStreak = 0
	s.windowStaticStreak = 0
	s.windowFingerprint = ""
	s.lastNoticeCode = ""
	s.lastNoticeAt = time.Time{}
	s.lastNoticeLogAt = time.Time{}
}

func (s *Service) restartMonitorContext() {
	s.mu.Lock()
	s.resetMonitorStateLocked()
	s.mu.Unlock()
	s.wakeMonitorLoop()
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

func (s *Service) monitorReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	active, _ := s.cfg.CurrentSavedAccount()
	if !s.cfg.AccountLogin || strings.TrimSpace(active.AccessKey) == "" {
		return false
	}
	return s.usesLoaderAPIConfig(s.cfg)
}

func (s *Service) prepareScanSession(ctx context.Context, silent bool) error {
	if s.monitorReady() {
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

	return nil
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

	ok, scanResult, consumeErr := s.consumeImageResult(ctx, img)
	if consumeErr == nil {
		if isExpiredScanResult(scanResult) {
			s.logf("scan reported expired QR; manual refresh is required")
			hint, hintErr := s.tryRefreshExpiredWindowQRCode(img)
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

func (s *Service) consumeImageResult(ctx context.Context, img image.Image) (bool, ScanResult, error) {
	ticket, rawURL, err := qr.DecodeTicketFromImage(img)
	if err != nil {
		return false, ScanResult{}, err
	}
	return s.consumeTicketResult(ctx, ticket, rawURL)
}

func (s *Service) consumeTicketResult(ctx context.Context, ticket, rawURL string) (bool, ScanResult, error) {
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

func (s *Service) login(ctx context.Context, account, password string, rememberPassword bool, cap map[string]any, openBrowser bool) (LoginResult, error) {
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
	requestedAccount := strings.TrimSpace(account)
	active, _ := cfg.CurrentSavedAccount()
	canReuseStoredPassword := requestedAccount == "" || strings.EqualFold(strings.TrimSpace(active.Account), requestedAccount)
	accountHint := requestedAccount
	if accountHint == "" {
		accountHint = strings.TrimSpace(active.Account)
	}
	if err := s.prepareBSGameSDK(ctx, accountHint); err != nil {
		return LoginResult{}, err
	}
	defer func() {
		if !keepPendingCredentials {
			s.clearPendingCredentials()
		}
	}()

	result := LoginResult{}

	uid := fmt.Sprintf("%d", active.UID)
	accessKey := active.AccessKey
	var userInfo map[string]any

	if active.LastLoginSucc && active.UID != 0 && active.AccessKey != "" {
		info, err := s.bili.GetUserInfo(ctx, uid, accessKey)
		if err == nil && config.StringValue(info["uname"]) != "" {
			userInfo = info
			result.UName = config.StringValue(info["uname"])
			result.SessionReady = true
		} else {
			s.mu.Lock()
			s.cfg.ClearSavedAccountSession(s.cfg.CurrentAccount)
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
			account = active.Account
		}
		if password == "" && canReuseStoredPassword {
			password = active.Password
		}
		if account == "" || password == "" {
			return result, localizedErrorf("backend.error.credentials_required", nil, "account and password are required")
		}
		s.storePendingCredentials(account, password, rememberPassword)

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
		saveErr := s.saveAccountStateLocked(account, password, rememberPassword, config.Int64Value(uid), accessKey, result.UName, true, false)
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
		result.UName = strings.TrimSpace(active.UName)
	}
	if result.UName == "" {
		result.UName = strings.TrimSpace(account)
	}

	result.SessionReady = true

	s.mu.Lock()
	saveErr := s.saveAccountStateLocked(account, password, rememberPassword, 0, accessKey, result.UName, true, true)
	s.mu.Unlock()
	if saveErr != nil {
		return result, saveErr
	}
	s.restartMonitorContext()

	s.mu.Lock()
	s.captchaPending = false
	s.captchaURL = ""
	s.mu.Unlock()
	if s.server != nil {
		s.server.ClearChallengeState()
	}

	result.OK = true
	result.MessageCode = "common.ok"
	result.Message = "ok"
	s.logf("login completed")
	s.emitState()
	return result, nil
}

func (s *Service) handleCaptchaResult(payload map[string]any) {
	account, password, rememberPassword, ok := s.pendingCredentials()
	if !ok {
		s.logf("captcha callback received but account credentials are missing")
		return
	}

	go func(account string, password []byte, rememberPassword bool) {
		defer wipeBytes(password)
		if _, err := s.login(context.Background(), account, string(password), rememberPassword, payload, false); err != nil {
			s.logf("captcha login continuation failed: %v", err)
		}
	}(account, password, rememberPassword)
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
	sameCode := s.lastNoticeCode == ref.Code
	shouldEmitState := !sameCode || now.Sub(s.lastNoticeAt) >= hintStateInterval
	shouldWriteLog := !sameCode || now.Sub(s.lastNoticeLogAt) >= hintLogInterval
	if !shouldEmitState && !shouldWriteLog {
		s.mu.Unlock()
		return
	}
	if shouldEmitState {
		s.lastError = text
		s.lastErrorMessage = cloneMessageRef(ref)
		s.lastNoticeCode = ref.Code
		s.lastNoticeAt = now
	}
	if shouldWriteLog {
		s.lastNoticeLogAt = now
	}
	s.mu.Unlock()

	if shouldWriteLog && s.fileLog != nil {
		s.fileLog.Writef("hint: %s", s.sanitizeMessage(text))
	}
	if shouldEmitState {
		s.emitState()
	}
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
	active, _ := s.cfg.CurrentSavedAccount()
	password := active.Password
	accessKey := active.AccessKey
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

func (s *Service) usesLoaderAPIConfig(cfg *config.Config) bool {
	return cfg != nil && strings.TrimSpace(cfg.LoaderAPIBaseURL) != ""
}

func (s *Service) ShouldUseBundledLoaderAPI() bool {
	s.mu.RLock()
	cfg := s.cfg.Clone()
	s.mu.RUnlock()
	return shouldUseBundledLoaderAPI(cfg)
}

func shouldUseBundledLoaderAPI(cfg *config.Config) bool {
	if cfg == nil {
		return true
	}
	current := strings.TrimSpace(cfg.LoaderAPIBaseURL)
	return current == "" || isLoopbackLoaderAPI(current)
}

func (s *Service) AdoptBundledLoaderAPI(baseURL string) error {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil
	}

	s.mu.Lock()
	if s.cfg == nil {
		s.mu.Unlock()
		return fmt.Errorf("config is not loaded")
	}

	current := strings.TrimSpace(s.cfg.LoaderAPIBaseURL)
	if current == baseURL {
		s.mu.Unlock()
		return nil
	}
	if current != "" && !isLoopbackLoaderAPI(current) {
		s.mu.Unlock()
		return nil
	}

	nextCfg := s.cfg.Clone()
	nextCfg.LoaderAPIBaseURL = baseURL
	err := config.Save(s.cfgPath, nextCfg)
	if err == nil {
		s.cfg = nextCfg
		s.apiReady = false
		s.lastError = ""
		s.lastErrorMessage = MessageRef{}
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}

	s.logf("adopted bundled endpoint %s", baseURL)
	s.emitState()
	return nil
}

func (s *Service) refreshAPIHealth(ctx context.Context) error {
	cfg := s.Config()
	if !s.usesLoaderAPIConfig(cfg) {
		s.setAPIReady(false)
		return localizedErrorf("backend.error.loader_api_required", nil, "loader api base url is required")
	}
	s.beginAPIInteraction()
	defer s.endAPIInteraction()
	err := bridge.ProbeLoaderAPI(ctx, cfg.LoaderAPIBaseURL, bridge.ClientMetaForBaseURL(cfg.LoaderAPIBaseURL))
	s.setAPIReady(err == nil)
	return err
}

func (s *Service) prepareBSGameSDK(ctx context.Context, accountHint string) error {
	deviceProfile, err := s.ensureLocalDeviceProfile(accountHint)
	if err != nil {
		return err
	}

	runtimeProfile := bridge.RuntimeProfile{}
	cfg := s.Config()
	if strings.TrimSpace(cfg.LoaderAPIBaseURL) != "" {
		if s.shouldDeferRuntimeProfileFetch(cfg.LoaderAPIBaseURL) {
			s.setAPIReady(false)
		} else {
			s.beginAPIInteraction()
			profile, profileErr := bridge.FetchRuntimeProfile(ctx, cfg.LoaderAPIBaseURL, bridge.ClientMetaForBaseURL(cfg.LoaderAPIBaseURL))
			s.endAPIInteraction()
			if profileErr != nil {
				s.setAPIReady(false)
				s.logf("runtime profile fetch failed; using fallback sdk constants: %v", profileErr)
			} else {
				s.setAPIReady(true)
				runtimeProfile = profile
			}
		}
	}

	s.bili.SetProfile(
		bsgamesdk.RuntimeProfile{
			ChannelID:      runtimeProfile.ChannelID,
			AppID:          runtimeProfile.AppID,
			CPID:           runtimeProfile.CPID,
			CPAppID:        runtimeProfile.CPAppID,
			CPAppKey:       runtimeProfile.CPAppKey,
			ServerID:       runtimeProfile.ServerID,
			ChannelVersion: runtimeProfile.ChannelVersion,
			GameVer:        runtimeProfile.GameVer,
			VersionCode:    runtimeProfile.VersionCode,
			SDKVer:         runtimeProfile.SDKVer,
		},
		bsgamesdk.DeviceProfile{
			Model:           deviceProfile.Model,
			Brand:           deviceProfile.Brand,
			SupportABIs:     deviceProfile.SupportABIs,
			Display:         deviceProfile.Display,
			AndroidID:       deviceProfile.AndroidID,
			MACAddress:      deviceProfile.MACAddress,
			IMEI:            deviceProfile.IMEI,
			RuntimeUDID:     deviceProfile.RuntimeUDID,
			UserProfileUDID: deviceProfile.UserProfileUDID,
			CurBuvid:        deviceProfile.CurBuvid,
		},
	)
	return nil
}

func (s *Service) shouldDeferRuntimeProfileFetch(baseURL string) bool {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return false
	}
	if !isLoopbackLoaderAPI(baseURL) {
		return false
	}
	return s.isRuntimePreparing()
}

func (s *Service) ensureLocalDeviceProfile(accountHint string) (config.DeviceProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg == nil {
		return config.DeviceProfile{}, fmt.Errorf("config is not loaded")
	}

	accountHint = strings.TrimSpace(accountHint)
	entry := config.SavedAccount{}
	if accountHint != "" {
		if existing, ok := s.cfg.FindSavedAccount(accountHint); ok {
			entry = existing
		} else {
			entry = config.SavedAccount{Account: accountHint}
		}
	} else if current, ok := s.cfg.CurrentSavedAccount(); ok {
		entry = current
		accountHint = strings.TrimSpace(current.Account)
	}

	if entry.DeviceProfile.IsComplete() {
		return entry.DeviceProfile, nil
	}

	baseProfile := entry.DeviceProfile
	if !baseProfile.IsComplete() && s.cfg.DeviceProfile.IsComplete() {
		baseProfile = s.cfg.DeviceProfile
	}

	profile, err := config.CompleteDeviceProfile(baseProfile)
	if err != nil {
		return config.DeviceProfile{}, err
	}
	nextCfg := s.cfg.Clone()
	if accountHint != "" {
		entry.Account = accountHint
		entry.DeviceProfile = profile
		nextCfg.UpsertSavedAccount(entry)
		nextCfg.DeviceProfile = config.DeviceProfile{}
	} else {
		nextCfg.DeviceProfile = profile
	}
	if err := config.Save(s.cfgPath, nextCfg); err != nil {
		return config.DeviceProfile{}, err
	}
	s.cfg = nextCfg
	return profile, nil
}

func (s *Service) setAPIReady(ready bool) {
	s.mu.Lock()
	changed := s.apiReady != ready
	s.apiReady = ready
	s.mu.Unlock()
	if changed {
		s.emitState()
	}
}

func (s *Service) setRuntimePreparing(preparing bool) {
	s.mu.Lock()
	changed := s.runtimePreparing != preparing
	s.runtimePreparing = preparing
	if preparing {
		s.lastAction = "runtime_starting"
	} else if s.lastAction == "runtime_starting" && s.running {
		s.lastAction = "monitoring"
	}
	s.mu.Unlock()
	if changed {
		s.emitState()
	}
}

func (s *Service) isRuntimePreparing() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runtimePreparing
}

func (s *Service) beginAPIInteraction() {
	s.mu.Lock()
	s.apiInteractionDepth++
	s.mu.Unlock()
}

func (s *Service) endAPIInteraction() {
	s.mu.Lock()
	if s.apiInteractionDepth > 0 {
		s.apiInteractionDepth--
	}
	s.mu.Unlock()
}

func (s *Service) isAPIInteractionActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.apiInteractionDepth > 0
}

func (s *Service) requireLoaderAPIConfigured() error {
	s.mu.RLock()
	cfg := s.cfg.Clone()
	s.mu.RUnlock()
	if s.usesLoaderAPIConfig(cfg) {
		return nil
	}
	s.setAPIReady(false)
	return localizedErrorf("backend.error.loader_api_required", nil, "loader api base url is required")
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

func (s *Service) storePendingCredentials(account, password string, rememberPassword bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingPasswordGen++
	s.clearPendingCredentialsLocked()
	s.pendingAccount = strings.TrimSpace(account)
	s.pendingPassword = append([]byte(nil), []byte(password)...)
	s.pendingRememberPass = rememberPassword
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
	s.pendingRememberPass = false
	s.pendingPasswordTTL = time.Time{}
}

func (s *Service) pendingCredentials() (string, []byte, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(s.pendingAccount) == "" || len(s.pendingPassword) == 0 {
		return "", nil, false, false
	}
	if !s.pendingPasswordTTL.IsZero() && time.Now().After(s.pendingPasswordTTL) {
		s.clearPendingCredentialsLocked()
		return "", nil, false, false
	}
	password := append([]byte(nil), s.pendingPassword...)
	return s.pendingAccount, password, s.pendingRememberPass, true
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

func managedBackgroundPathFor(contentType string) (string, error) {
	// Store managed background in the executable directory so packaged apps carry it alongside the binary.
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, managedBackgroundBaseName+backgroundExtensionFor(contentType)), nil
}

func backgroundExtensionFor(contentType string) string {
	switch contentType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".img"
	}
}

func detectBackgroundContentType(path string, data []byte) (string, error) {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(path)))
	if ext == ".webp" {
		return "image/webp", nil
	}
	contentType := strings.ToLower(strings.TrimSpace(mime.TypeByExtension(ext)))
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	if contentType == "" {
		contentType = strings.ToLower(strings.TrimSpace(http.DetectContentType(data)))
		contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}
	switch contentType {
	case "image/png", "image/jpeg", "image/webp":
		return contentType, nil
	default:
		return "", fmt.Errorf("unsupported background image format (use png, jpg, jpeg, or webp)")
	}
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
	contentType, err := detectBackgroundContentType(path, data)
	if err != nil {
		return "", err
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
	contentType, err := detectBackgroundContentType(sourcePath, data)
	if err != nil {
		return "", "", err
	}

	destAbsPath, err := managedBackgroundPathFor(contentType)
	if err != nil {
		return "", "", err
	}
	if err := config.AtomicWriteFile(destAbsPath, data, 0o644); err != nil {
		return "", "", fmt.Errorf("write background image: %w", err)
	}

	if previousPath != "" && !samePath(previousPath, destAbsPath) &&
		strings.HasPrefix(strings.ToLower(filepath.Base(previousPath)), managedBackgroundBaseName+".") {
		_ = os.Remove(previousPath)
	}

	sum := sha256.Sum256(data)
	s.logf(
		"background image imported as %s (%s, %d bytes)",
		destAbsPath,
		fmt.Sprintf("%x", sum[:8]),
		len(data),
	)

	// Store relative path in config for portability (relative to executable dir)
	rel := "./" + filepath.Base(destAbsPath)
	return rel, "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
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
