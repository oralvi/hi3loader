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

var (
	authVerifyEndpoint  = sdkEndpoint("granter", "login", "v2", "login")
	ticketInspectTarget = sdkEndpoint("panda", "qrcode", "scan")
	ticketConfirmTarget = sdkEndpoint("panda", "qrcode", "confirm")
)

type Client struct {
	http *http.Client

	mu            sync.RWMutex
	hasDispatch   bool
	localDispatch map[string]any
	hasBHVer      bool
	localBHVer    string
}

type SessionInfo struct {
	UID        string
	OpenID     string
	ComboID    string
	ComboToken string
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
	data, err := parseJSONMap(verifyDataR)
	if err != nil {
		return nil, fmt.Errorf("parse verify data template: %w", err)
	}
	if numericUID := config.Int64Value(uid); numericUID != 0 {
		data["uid"] = numericUID
	} else {
		data["uid"] = uid
	}
	data["access_key"] = accessKey

	body, err := parseJSONMap(verifyBodyR)
	if err != nil {
		return nil, fmt.Errorf("parse verify body template: %w", err)
	}
	encodedData, err := compactJSON(data)
	if err != nil {
		return nil, fmt.Errorf("encode verify data: %w", err)
	}
	body["data"] = encodedData
	body = makeSign(body)

	resp := map[string]any{}
	encodedBody, err := compactJSON(body)
	if err != nil {
		return nil, fmt.Errorf("encode verify body: %w", err)
	}
	err = netutil.PostBodyJSON(ctx, c.http, authVerifyEndpoint, encodedBody, jsonHeaders(), &resp)
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

	if cfg != nil && cfg.BHVer != "" {
		c.mu.Lock()
		c.localBHVer = cfg.BHVer
		c.hasBHVer = true
		c.mu.Unlock()
		return cfg.BHVer, nil
	}

	return "", nil
}

func (c *Client) GetOAServer(ctx context.Context, uid string, cfg *config.Config) (map[string]any, error) {
	officialMode := isOfficialDispatchURL(effectiveDispatchURL(cfg))

	c.mu.RLock()
	if c.hasDispatch {
		defer c.mu.RUnlock()
		return cloneMap(c.localDispatch), nil
	}
	c.mu.RUnlock()

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

	if cfg != nil {
		cacheKey := dispatchCacheKey(bhVer)
		if snapshotVersion, entry, ok := cfg.DispatchSnapshot(); ok && snapshotVersion == cacheKey && LooksLikeFinalDispatch(entry.Data) {
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

	resp, err := c.fetchDispatch(ctx, bhVer, uid, cfg)
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
	return config.NormalizeDispatchVersion(version)
}

func (c *Client) ScanCheck(ctx context.Context, session SessionInfo, ticket string, cfg *config.Config) (map[string]any, error) {
	check, err := parseJSONMap(scanCheckR)
	if err != nil {
		return nil, fmt.Errorf("parse scan check template: %w", err)
	}
	check["ticket"] = ticket
	check["ts"] = int(time.Now().Unix())
	check = makeSign(check)

	scanResp := map[string]any{}
	encodedCheck, err := compactJSON(check)
	if err != nil {
		return nil, fmt.Errorf("encode scan check payload: %w", err)
	}
	if err := netutil.PostBodyJSON(ctx, c.http, ticketInspectTarget, encodedCheck, jsonHeaders(), &scanResp); err != nil {
		return nil, err
	}
	if config.Int64Value(scanResp["retcode"]) != 0 {
		return scanResp, nil
	}

	return c.ScanConfirm(ctx, session, ticket, cfg)
}

func (c *Client) ScanConfirm(ctx context.Context, session SessionInfo, ticket string, cfg *config.Config) (map[string]any, error) {
	scanResult, err := parseJSONMap(scanResultR)
	if err != nil {
		return nil, fmt.Errorf("parse scan result template: %w", err)
	}
	scanData, err := parseJSONMap(scanDataR)
	if err != nil {
		return nil, fmt.Errorf("parse scan data template: %w", err)
	}

	dispatch, err := c.GetOAServer(ctx, session.UID, cfg)
	if err != nil {
		return nil, err
	}
	if dispatchData, ok := dispatch["data"]; ok {
		scanData["dispatch"] = dispatchData
	}
	scanData["accountID"] = session.OpenID
	scanData["accountToken"] = session.ComboToken

	scanExt, err := parseJSONMap(scanExtR)
	if err != nil {
		return nil, fmt.Errorf("parse scan ext template: %w", err)
	}
	scanExt["data"] = scanData

	scanRaw, err := parseJSONMap(scanRawR)
	if err != nil {
		return nil, fmt.Errorf("parse scan raw template: %w", err)
	}
	scanRaw["open_id"] = session.OpenID
	scanRaw["combo_id"] = session.ComboID
	scanRaw["combo_token"] = session.ComboToken

	scanPayload, err := parseJSONMap(scanPayloadR)
	if err != nil {
		return nil, fmt.Errorf("parse scan payload template: %w", err)
	}
	encodedRaw, err := compactJSON(scanRaw)
	if err != nil {
		return nil, fmt.Errorf("encode scan raw payload: %w", err)
	}
	encodedExt, err := compactJSON(scanExt)
	if err != nil {
		return nil, fmt.Errorf("encode scan ext payload: %w", err)
	}
	scanPayload["raw"] = encodedRaw
	scanPayload["ext"] = encodedExt

	scanResult["payload"] = scanPayload
	scanResult["ts"] = int(time.Now().Unix())
	scanResult["ticket"] = ticket
	scanResult = makeSign(scanResult)

	resp := map[string]any{}
	encodedScanResult, err := compactJSON(scanResult)
	if err != nil {
		return nil, fmt.Errorf("encode scan confirm payload: %w", err)
	}
	err = netutil.PostBodyJSON(ctx, c.http, ticketConfirmTarget, encodedScanResult, jsonHeaders(), &resp)
	return resp, err
}

func signPayload(data string) string {
	mac := hmac.New(sha256.New, signatureMaterial())
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
	data["sign"] = signPayload(signBase)
	return data
}

func effectiveVersionURL(cfg *config.Config) string {
	if cfg != nil && cfg.VersionAPI != "" {
		return cfg.VersionAPI
	}
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

func (c *Client) fetchDispatch(ctx context.Context, version, uid string, cfg *config.Config) (map[string]any, error) {
	baseURL := effectiveDispatchURL(cfg)
	if isOfficialDispatchURL(baseURL) {
		token := ""
		if cfg != nil {
			token = strings.TrimSpace(cfg.BILIHITOKEN)
		}
		if token == "" {
			attempts := 3
			var lastErr error
			for i := 0; i < attempts; i++ {
				info, err := bilihitoken.FetchReleaseInfo(c.http)
				if err == nil {
					t, err2 := bilihitoken.FetchCredential(c.http, info.PackageURL)
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
				return nil, fmt.Errorf("failed to refresh required credential automatically: %v; please set it in config", lastErr)
			}
		}
		uid = strings.TrimSpace(uid)
		if uid == "" {
			return nil, fmt.Errorf("required identifier is unavailable")
		}
		target, err := buildOfficialDispatchURL(baseURL, version, token, uid)
		if err != nil {
			return nil, err
		}
		raw, err := netutil.GetText(ctx, c.http, target, nil)
		if err != nil {
			return nil, err
		}
		resp, err := parseDispatchResponse(raw)
		if err != nil {
			return nil, err
		}
		resp["source"] = officialDispatchName
		resp["request_mode"] = "credential_flow"
		addDispatchBlobSummary(resp)
		return resp, nil
	}

	raw, err := netutil.GetText(ctx, c.http, buildDispatchURL(baseURL, version), nil)
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

func sdkEndpoint(parts ...string) string {
	base := strings.Join([]string{"https://", "api-sdk", ".mihoyo.com", "/bh3_cn/combo/"}, "")
	return base + strings.Join(parts, "/")
}

func signatureMaterial() []byte {
	return []byte(strings.Join([]string{"0ebc517adb1b62c6", "b408df153331f9aa"}, ""))
}

func compactJSON(v any) (string, error) {
	buf, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal json: %w", err)
	}
	return strings.ReplaceAll(string(buf), " ", ""), nil
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

func parseJSONMap(raw string) (map[string]any, error) {
	out := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse json map: %w", err)
	}
	return out, nil
}

func ExtractSessionInfo(resp map[string]any) (*SessionInfo, error) {
	if resp == nil {
		return nil, fmt.Errorf("verify response is empty")
	}
	dataAny, ok := resp["data"]
	if !ok {
		return nil, fmt.Errorf("verify response missing data")
	}
	data, ok := dataAny.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("verify response data has unexpected type %T", dataAny)
	}

	session := &SessionInfo{
		UID:        strings.TrimSpace(config.StringValue(data["uid"])),
		OpenID:     strings.TrimSpace(config.StringValue(data["open_id"])),
		ComboID:    strings.TrimSpace(config.StringValue(data["combo_id"])),
		ComboToken: strings.TrimSpace(config.StringValue(data["combo_token"])),
	}
	if session.OpenID == "" {
		return nil, fmt.Errorf("verify response missing open_id")
	}
	if session.ComboToken == "" {
		return nil, fmt.Errorf("verify response missing combo_token")
	}
	return session, nil
}
