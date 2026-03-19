package bridge

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"hi3loader/internal/config"
	"hi3loader/internal/mihoyosdk"
)

const helperArg = "--aux-runtime"
const helperTokenEnv = "HI3LOADER_AUX_TOKEN"
const helperAuthFileEnv = "HI3LOADER_AUX_AUTH"

func NewSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate helper token: %w", err)
	}
	return base64.RawStdEncoding.EncodeToString(buf), nil
}

type Operation string

const (
	opLogin            Operation = "login"
	opVerifySession    Operation = "verify_session"
	opFetchReleaseInfo Operation = "fetch_release_info"
	opFetchCredential  Operation = "fetch_credential"
	opResolveDispatch  Operation = "resolve_dispatch"
	opScanCheck        Operation = "scan_check"
)

type Request struct {
	Op       Operation        `json:"op"`
	Login    *LoginRequest    `json:"login,omitempty"`
	Verify   *VerifyRequest   `json:"verify,omitempty"`
	Dispatch *DispatchRequest `json:"dispatch,omitempty"`
	Scan     *ScanRequest     `json:"scan,omitempty"`
}

type Response struct {
	Error       string               `json:"error,omitempty"`
	Login       *LoginResponse       `json:"login,omitempty"`
	Verify      *VerifyResponse      `json:"verify,omitempty"`
	ReleaseInfo *ReleaseInfoResponse `json:"releaseInfo,omitempty"`
	Credential  *CredentialResponse  `json:"credential,omitempty"`
	Dispatch    *DispatchResponse    `json:"dispatch,omitempty"`
	Scan        *ScanResponse        `json:"scan,omitempty"`
}

type CaptchaPayload struct {
	Challenge string `json:"challenge,omitempty"`
	Validate  string `json:"validate,omitempty"`
	UserID    string `json:"userId,omitempty"`
}

type LoginRequest struct {
	Account       string          `json:"account,omitempty"`
	Password      string          `json:"password,omitempty"`
	UID           int64           `json:"uid,omitempty"`
	AccessKey     string          `json:"accessKey,omitempty"`
	LastLoginSucc bool            `json:"lastLoginSucc,omitempty"`
	Captcha       *CaptchaPayload `json:"captcha,omitempty"`
}

type LoginResponse struct {
	Message          string                `json:"message,omitempty"`
	NeedsCaptcha     bool                  `json:"needsCaptcha,omitempty"`
	CaptchaGT        string                `json:"captchaGT,omitempty"`
	CaptchaChallenge string                `json:"captchaChallenge,omitempty"`
	CaptchaUserID    string                `json:"captchaUserID,omitempty"`
	UID              int64                 `json:"uid,omitempty"`
	AccessKey        string                `json:"accessKey,omitempty"`
	UName            string                `json:"uname,omitempty"`
	VerifyRetcode    int64                 `json:"verifyRetcode,omitempty"`
	Session          mihoyosdk.SessionInfo `json:"session"`
}

type VerifyRequest struct {
	UID       int64  `json:"uid"`
	AccessKey string `json:"accessKey"`
}

type VerifyResponse struct {
	Retcode int64                 `json:"retcode,omitempty"`
	Message string                `json:"message,omitempty"`
	Session mihoyosdk.SessionInfo `json:"session"`
}

type ReleaseInfoResponse struct {
	Version int    `json:"version,omitempty"`
	BHVer   string `json:"bhVer,omitempty"`
}

type CredentialResponse struct {
	Token   string `json:"token,omitempty"`
	Version int    `json:"version,omitempty"`
	BHVer   string `json:"bhVer,omitempty"`
}

type ConfigSnapshot struct {
	GamePath              string `json:"gamePath,omitempty"`
	BHVer                 string `json:"bhVer,omitempty"`
	BiliPkgVer            int    `json:"biliPkgVer,omitempty"`
	VersionAPI            string `json:"versionApi,omitempty"`
	DispatchAPI           string `json:"dispatchApi,omitempty"`
	DispatchData          string `json:"dispatchData,omitempty"`
	DispatchVersion       string `json:"dispatchVersion,omitempty"`
	DispatchSource        string `json:"dispatchSource,omitempty"`
	DispatchRawLen        int    `json:"dispatchRawLen,omitempty"`
	DispatchDecodedLen    int    `json:"dispatchDecodedLen,omitempty"`
	DispatchDecodedSHA256 string `json:"dispatchDecodedSHA256,omitempty"`
	DispatchSavedAt       string `json:"dispatchSavedAt,omitempty"`
	BILIHITOKEN           string `json:"biliHitoken,omitempty"`
	HI3UID                string `json:"hi3uid,omitempty"`
}

func (s ConfigSnapshot) ToConfig() *config.Config {
	cfg := config.Default()
	cfg.GamePath = s.GamePath
	cfg.BHVer = s.BHVer
	cfg.BiliPkgVer = s.BiliPkgVer
	cfg.VersionAPI = s.VersionAPI
	cfg.DispatchAPI = s.DispatchAPI
	cfg.DispatchData = s.DispatchData
	cfg.DispatchVersion = s.DispatchVersion
	cfg.DispatchSource = s.DispatchSource
	cfg.DispatchRawLen = s.DispatchRawLen
	cfg.DispatchDecodedLen = s.DispatchDecodedLen
	cfg.DispatchDecodedSHA256 = s.DispatchDecodedSHA256
	cfg.DispatchSavedAt = s.DispatchSavedAt
	cfg.BILIHITOKEN = s.BILIHITOKEN
	cfg.HI3UID = s.HI3UID
	cfg.Normalize()
	return cfg
}

type BlobSummary struct {
	RawLen        int    `json:"rawLen,omitempty"`
	DecodedLen    int    `json:"decodedLen,omitempty"`
	DecodedSHA256 string `json:"decodedSHA256,omitempty"`
}

type DispatchRequest struct {
	Config  ConfigSnapshot `json:"config"`
	UID     string         `json:"uid,omitempty"`
	Version string         `json:"version,omitempty"`
}

type DispatchResponse struct {
	Retcode       int64       `json:"retcode,omitempty"`
	Message       string      `json:"message,omitempty"`
	Data          string      `json:"data,omitempty"`
	Source        string      `json:"source,omitempty"`
	CachedSource  string      `json:"cachedSource,omitempty"`
	CachedSavedAt string      `json:"cachedSavedAt,omitempty"`
	BlobSummary   BlobSummary `json:"blobSummary"`
}

type ScanRequest struct {
	Config  ConfigSnapshot        `json:"config"`
	Session mihoyosdk.SessionInfo `json:"session"`
	Ticket  string                `json:"ticket,omitempty"`
}

type ScanResponse struct {
	Retcode int64  `json:"retcode,omitempty"`
	Message string `json:"message,omitempty"`
}
