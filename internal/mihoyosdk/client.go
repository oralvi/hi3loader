package mihoyosdk

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"hi3loader/internal/bilihitoken"
	"hi3loader/internal/config"
	"hi3loader/internal/gameclient"
	"hi3loader/internal/netutil"
)

const (
	verifyURL  = "https://api-sdk.mihoyo.com/bh3_cn/combo/granter/login/v2/login"
	scanURL    = "https://api-sdk.mihoyo.com/bh3_cn/combo/panda/qrcode/scan"
	confirmURL = "https://api-sdk.mihoyo.com/bh3_cn/combo/panda/qrcode/confirm"
	// versionURL removed: do not call remote version API by default
)

type Client struct {
	http *http.Client

	mu            sync.RWMutex
	hasDispatch   bool
	localDispatch map[string]any
	hasBHVer      bool
	localBHVer    string
}

func NewClient() *Client {
	return &Client{
		http:          netutil.NewClient(),
		localDispatch: map[string]any{},
		localBHVer:    "5.8.0",
	}
}

func (c *Client) ResetCache() {
	c.mu.Lock()
	c.hasDispatch = false
	c.localDispatch = map[string]any{}
	c.hasBHVer = false
	c.localBHVer = "5.8.0"
	c.mu.Unlock()
}

func (c *Client) ResetDispatchCache() {
	c.mu.Lock()
	c.hasDispatch = false
	c.localDispatch = map[string]any{}
	c.mu.Unlock()
}

func (c *Client) Verify(ctx context.Context, uid, accessKey string) (map[string]any, error) {
	data := mustJSONMap(verifyDataR)
	if numericUID := config.Int64Value(uid); numericUID != 0 {
		data["uid"] = numericUID
	} else {
		data["uid"] = uid
	}
	data["access_key"] = accessKey

	body := mustJSONMap(verifyBodyR)
	body["data"] = compactJSON(data)
	body = makeSign(body)

	resp := map[string]any{}
	err := netutil.PostBodyJSON(ctx, c.http, verifyURL, compactJSON(body), jsonHeaders(), &resp)
	return resp, err
}

func (c *Client) GetBHVer(ctx context.Context, cfg *config.Config) (string, error) {
	if cfg != nil && cfg.GamePath != "" {
		if version, err := gameclient.ReadVersion(cfg.GamePath); err == nil && version != "" {
			c.mu.Lock()
			c.localBHVer = version
			c.hasBHVer = true
			c.mu.Unlock()
			return version, nil
		}
		if cfg.BHVer != "" {
			c.mu.Lock()
			c.localBHVer = cfg.BHVer
			c.hasBHVer = true
			c.mu.Unlock()
			return cfg.BHVer, nil
		}
	}

	c.mu.RLock()
	if c.hasBHVer {
		defer c.mu.RUnlock()
		return c.localBHVer, nil
	}
	c.mu.RUnlock()

	// Try remote version first if configured
	if verURL := effectiveVersionURL(cfg); verURL != "" {
		resp := map[string]any{}
		if err := netutil.GetJSON(ctx, c.http, verURL, nil, &resp); err == nil {
			version := config.StringValue(resp["version"])
			if version != "" {
				c.mu.Lock()
				c.localBHVer = version
				c.hasBHVer = true
				c.mu.Unlock()
				return version, nil
			}
		}
	}

	// Fallback to local configuration or game directory values
	if cfg != nil && cfg.BHVer != "" {
		c.mu.Lock()
		c.localBHVer = cfg.BHVer
		c.hasBHVer = true
		c.mu.Unlock()
		return cfg.BHVer, nil
	}

	// If game path resolved earlier, try reading it again (handled above), otherwise return empty
	return "", nil
}

func (c *Client) GetOAServer(ctx context.Context, openID, comboToken, uid string, cfg *config.Config) (map[string]any, error) {
	officialMode := isOfficialDispatchURL(effectiveDispatchURL(cfg))

	c.mu.RLock()
	if c.hasDispatch {
		defer c.mu.RUnlock()
		return cloneMap(c.localDispatch), nil
	}
	c.mu.RUnlock()

	// In official mode we intentionally avoid short-circuiting with local dispatch_data,
	// because it may have been populated by historical third-party fallback blobs.
	if cfg != nil && !officialMode && LooksLikeFinalDispatch(cfg.DispatchData) {
		resp := map[string]any{
			"retcode": 0,
			"message": "OK",
			"data":    cfg.DispatchData,
			"source":  "local_config",
		}
		addDispatchBlobSummary(resp)
		c.mu.Lock()
		c.localDispatch = cloneMap(resp)
		c.hasDispatch = true
		c.mu.Unlock()
		return resp, nil
	}

	bhVer, err := c.GetBHVer(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if cfg != nil && cfg.DispatchCache != nil {
		if entry, ok := cfg.DispatchCache[dispatchCacheKey(bhVer)]; ok && LooksLikeFinalDispatch(entry.Data) {
			// In official mode, skip cache entries originating from third-party/custom sources.
			if officialMode {
				source := strings.TrimSpace(entry.Source)
				if source == "" || source == customDispatchName {
					goto skipCachedDispatch
				}
			}
			resp := map[string]any{
				"retcode":       0,
				"message":       "OK",
				"data":          entry.Data,
				"source":        "local_cache",
				"cached_source": entry.Source,
				"cached_saved":  entry.SavedAt,
			}
			addDispatchBlobSummary(resp)
			c.mu.Lock()
			c.localDispatch = cloneMap(resp)
			c.hasDispatch = true
			c.mu.Unlock()
			return resp, nil
		}
	}
skipCachedDispatch:

	resp, err := c.fetchDispatch(ctx, bhVer, openID, uid, cfg)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.localDispatch = cloneMap(resp)
	c.hasDispatch = true
	c.mu.Unlock()
	return resp, nil
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

func (c *Client) ScanCheck(ctx context.Context, bhInfo map[string]any, ticket string, cfg *config.Config) (map[string]any, error) {
	check := mustJSONMap(scanCheckR)
	check["ticket"] = ticket
	check["ts"] = int(time.Now().Unix())
	check = makeSign(check)

	scanResp := map[string]any{}
	if err := netutil.PostBodyJSON(ctx, c.http, scanURL, compactJSON(check), jsonHeaders(), &scanResp); err != nil {
		return nil, err
	}
	if config.Int64Value(scanResp["retcode"]) != 0 {
		return scanResp, nil
	}

	return c.ScanConfirm(ctx, bhInfo, ticket, cfg)
}

func (c *Client) ScanConfirm(ctx context.Context, bhInfoResp map[string]any, ticket string, cfg *config.Config) (map[string]any, error) {
	bhInfoAny, ok := bhInfoResp["data"]
	if !ok {
		return nil, fmt.Errorf("verify response missing data")
	}
	bhInfo, ok := bhInfoAny.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("verify response data has unexpected type %T", bhInfoAny)
	}

	scanResult := mustJSONMap(scanResultR)
	scanData := mustJSONMap(scanDataR)

	dispatch, err := c.GetOAServer(
		ctx,
		config.StringValue(bhInfo["open_id"]),
		config.StringValue(bhInfo["combo_token"]),
		config.StringValue(bhInfo["uid"]),
		cfg,
	)
	if err != nil {
		return nil, err
	}
	if dispatchData, ok := dispatch["data"]; ok {
		scanData["dispatch"] = dispatchData
	}
	scanData["accountID"] = config.StringValue(bhInfo["open_id"])
	scanData["accountToken"] = config.StringValue(bhInfo["combo_token"])

	scanExt := mustJSONMap(scanExtR)
	scanExt["data"] = scanData

	scanRaw := mustJSONMap(scanRawR)
	scanRaw["open_id"] = config.StringValue(bhInfo["open_id"])
	scanRaw["combo_id"] = config.StringValue(bhInfo["combo_id"])
	scanRaw["combo_token"] = config.StringValue(bhInfo["combo_token"])

	scanPayload := mustJSONMap(scanPayloadR)
	scanPayload["raw"] = compactJSON(scanRaw)
	scanPayload["ext"] = compactJSON(scanExt)

	scanResult["payload"] = scanPayload
	scanResult["ts"] = int(time.Now().Unix())
	scanResult["ticket"] = ticket
	scanResult = makeSign(scanResult)

	resp := map[string]any{}
	err = netutil.PostBodyJSON(ctx, c.http, confirmURL, compactJSON(scanResult), jsonHeaders(), &resp)
	return resp, err
}

func BH3Sign(data string) string {
	mac := hmac.New(sha256.New, []byte("0ebc517adb1b62c6b408df153331f9aa"))
	_, _ = mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func makeSign(data map[string]any) map[string]any {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		if key == "sign" {
			continue
		}
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(config.StringValue(data[key]))
		builder.WriteString("&")
	}
	signBase := strings.TrimRight(builder.String(), "&")
	signBase = strings.ReplaceAll(signBase, " ", "")
	data["sign"] = BH3Sign(signBase)
	return data
}

func effectiveVersionURL(cfg *config.Config) string {
	if cfg != nil && cfg.VersionAPI != "" {
		return cfg.VersionAPI
	}
	// Do not use the hard-coded remote version URL by default.
	return ""
}

func effectiveDispatchURL(cfg *config.Config) string {
	if cfg != nil && cfg.DispatchAPI != "" {
		return cfg.DispatchAPI
	}
	return defaultDispatchURL
}

func buildDispatchURL(baseURL, version string) string {
	query := "version=" + version + "_gf_android_bilibili&t=" + config.StringValue(time.Now().Unix())
	switch {
	case strings.Contains(baseURL, "?") && strings.HasSuffix(baseURL, "?"):
		return baseURL + query
	case strings.Contains(baseURL, "?") && strings.HasSuffix(baseURL, "&"):
		return baseURL + query
	case strings.Contains(baseURL, "?"):
		return baseURL + "&" + query
	default:
		return baseURL + "?" + query
	}
}

// official dispatch helpers moved to official_dispatch.go

func (c *Client) fetchDispatch(ctx context.Context, version, openID, uid string, cfg *config.Config) (map[string]any, error) {
	baseURL := effectiveDispatchURL(cfg)
	if isOfficialDispatchURL(baseURL) {
		// Require explicit token and uid (no openID fallback).
		// Use cfg.BILIHITOKEN (APK-extracted client token).
		// If missing, attempt to fetch it (up to 3 tries, 5s interval). comboToken is ignored for official path.
		token := ""
		if cfg != nil {
			token = strings.TrimSpace(cfg.BILIHITOKEN)
		}
		if token == "" {
			// Try up to 3 times to fetch from APK
			attempts := 3
			var lastErr error
			for i := 0; i < attempts; i++ {
				info, err := bilihitoken.FetchGameInfo(c.http)
				if err == nil {
					t, err2 := bilihitoken.FetchDispatchToken(c.http, info.APKUrl)
					if err2 == nil && strings.TrimSpace(t) != "" {
						token = strings.TrimSpace(t)
						break
					}
					lastErr = err2
				} else {
					lastErr = err
				}
				if i < attempts-1 {
					time.Sleep(5 * time.Second)
				}
			}
			if token == "" {
				return nil, fmt.Errorf("failed to obtain BILIHITOKEN automatically: %v; please set BILIHITOKEN in config", lastErr)
			}
		}
		uid = strings.TrimSpace(uid)
		if uid == "" {
			return nil, fmt.Errorf("official dispatch requires uid or HI3UID")
		}
		target, err := buildOfficialDispatchURL(baseURL, version, token, uid)
		if err != nil {
			return nil, err
		}
		raw, err := netutil.GetText(ctx, c.http, target, dispatchHeaders(version, openID))
		if err != nil {
			return nil, err
		}
		resp, err := parseDispatchResponse(raw)
		if err != nil {
			return nil, err
		}
		resp["source"] = officialDispatchName
		resp["request_mode"] = "hi3uid_biliHitoken"
		addDispatchBlobSummary(resp)
		return resp, nil
	}

	raw, err := netutil.GetText(ctx, c.http, buildDispatchURL(baseURL, version), dispatchHeaders(version, openID))
	if err != nil {
		return nil, err
	}
	resp, err := parseDispatchResponse(raw)
	if err != nil {
		return nil, err
	}
	resp["source"] = customDispatchName
	addDispatchBlobSummary(resp)
	return resp, nil
}

// (FetchBothDispatches removed - restoring original behavior)

func dispatchHeaders(version, openID string) map[string]string {
	signParam := "x-req-code=80&x-req-name=pc-1.4.9:80&x-req-openid=" + openID + "&x-req-version=" + version + "_gf_android_bilibili"
	return map[string]string{
		"x-req-code":    "80",
		"x-req-name":    "pc-1.4.9:80",
		"x-req-openid":  openID,
		"x-req-version": version + "_gf_android_bilibili",
		"x-req-sign":    BH3Sign(signParam),
	}
}

func compactJSON(v any) string {
	buf, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return strings.ReplaceAll(string(buf), " ", "")
}

func jsonHeaders() map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
	}
}

func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func mustJSONMap(raw string) map[string]any {
	out := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		panic(err)
	}
	return out
}

const verifyBodyR = `{"device":"0000000000000000","app_id":"1","channel_id":"14","data":{},"sign":""}`
const verifyDataR = `{"uid":1,"access_key":"590"}`
const scanResultR = `{"device":"0000000000000000","app_id":1,"ts":1637593776681,"ticket":"","payload":{},"sign":""}`
const scanPayloadR = `{"raw":"","proto":"Combo","ext":""}`
const scanRawR = `{"heartbeat":false,"open_id":"","device_id":"0000000000000000","app_id":"1","channel_id":"14","combo_token":"","asterisk_name":"66666666","combo_id":"","account_type":"2"}`
const scanExtR = `{"data":{}}`
const scanDataR = `{"accountType":"2","accountID":"","accountToken":"","dispatch":{}}`
const scanCheckR = `{"app_id":"1","device":"0000000000000000","ticket":"abab","ts":1637593776066,"sign":"abab"}`
