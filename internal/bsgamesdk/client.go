package bsgamesdk

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"hi3loader/internal/config"
	"hi3loader/internal/netutil"
)

var providerBase = providerBaseURL()

type Client struct {
	http *http.Client
	mu   sync.RWMutex

	runtimeProfile RuntimeProfile
	deviceProfile  DeviceProfile
}

type payloadTemplate struct {
	data  map[string]any
	order []string
}

func NewClient() *Client {
	return &Client{http: netutil.NewClient()}
}

func (c *Client) SetProfile(runtime RuntimeProfile, device DeviceProfile) {
	c.mu.Lock()
	c.runtimeProfile = runtime
	c.deviceProfile = device
	c.mu.Unlock()
}

func (c *Client) GetUserInfo(ctx context.Context, uid, accessKey string) (map[string]any, error) {
	profile := c.effectiveProfile()
	payload := userProfileTemplate(profile)
	payload.data["uid"] = uid
	payload.data["access_key"] = accessKey
	body := payload.SetSign(profile.SignatureMaterial)

	resp := map[string]any{}
	err := netutil.PostBodyJSON(ctx, c.http, providerBase+"/api/client/user.info", body, defaultHeaders(), &resp)
	return resp, err
}

func (c *Client) Login(ctx context.Context, account, password string, cap map[string]any) (map[string]any, error) {
	var (
		resp map[string]any
		err  error
	)
	if cap != nil {
		resp, err = c.loginWithCaptcha(ctx, account, password, cap)
	} else {
		resp, err = c.loginWithoutCaptcha(ctx, account, password)
	}
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) StartCaptcha(ctx context.Context) (map[string]any, error) {
	profile := c.effectiveProfile()
	payload := challengeTemplate(profile)
	body := payload.SetSign(profile.SignatureMaterial)

	resp := map[string]any{}
	err := netutil.PostBodyJSON(ctx, c.http, providerBase+"api/client/start_captcha", body, defaultHeaders(), &resp)
	return resp, err
}

func MakeCaptchaURL(callbackAddr, gt, challenge, userID string) string {
	callbackAddr = strings.TrimSpace(callbackAddr)
	if callbackAddr == "" {
		callbackAddr = "127.0.0.1:0"
	}
	return fmt.Sprintf("http://%s/?captcha_type=1&challenge=%s&gt=%s&userid=%s&gs=1", callbackAddr, challenge, gt, userID)
}

func (c *Client) loginWithoutCaptcha(ctx context.Context, account, password string) (map[string]any, error) {
	profile := c.effectiveProfile()
	rsaResp, err := c.requestRSA(ctx)
	if err != nil {
		return nil, err
	}

	payload := credentialTemplate(profile)
	payload.data["access_key"] = ""
	payload.data["gt_user_id"] = ""
	payload.data["uid"] = ""
	payload.data["challenge"] = ""
	payload.data["user_id"] = account
	payload.data["validate"] = ""

	encrypted, err := rsaCreate(config.StringValue(rsaResp["hash"])+password, config.StringValue(rsaResp["rsa_key"]))
	if err != nil {
		return nil, err
	}
	payload.data["pwd"] = encrypted
	body := payload.SetSign(profile.SignatureMaterial)

	resp := map[string]any{}
	err = netutil.PostBodyJSON(ctx, c.http, providerBase+"api/client/login", body, defaultHeaders(), &resp)
	return resp, err
}

func (c *Client) loginWithCaptcha(ctx context.Context, account, password string, cap map[string]any) (map[string]any, error) {
	profile := c.effectiveProfile()
	rsaResp, err := c.requestRSA(ctx)
	if err != nil {
		return nil, err
	}

	payload := credentialTemplate(profile)
	payload.data["access_key"] = ""
	payload.data["gt_user_id"] = config.StringValue(cap["userid"])
	payload.data["uid"] = ""
	payload.data["challenge"] = config.StringValue(cap["challenge"])
	payload.data["user_id"] = account
	payload.data["validate"] = config.StringValue(cap["validate"])
	payload.data["seccode"] = config.StringValue(cap["validate"]) + "|jordan"

	encrypted, err := rsaCreate(config.StringValue(rsaResp["hash"])+password, config.StringValue(rsaResp["rsa_key"]))
	if err != nil {
		return nil, err
	}
	payload.data["pwd"] = encrypted
	body := payload.SetSign(profile.SignatureMaterial)

	resp := map[string]any{}
	err = netutil.PostBodyJSON(ctx, c.http, providerBase+"api/client/login", body, defaultHeaders(), &resp)
	return resp, err
}

func (c *Client) requestRSA(ctx context.Context) (map[string]any, error) {
	profile := c.effectiveProfile()
	payload := keyTemplate(profile)
	body := payload.SetSign(profile.SignatureMaterial)

	resp := map[string]any{}
	err := netutil.PostBodyJSON(ctx, c.http, providerBase+"api/client/rsa", body, defaultHeaders(), &resp)
	return resp, err
}

func defaultHeaders() map[string]string {
	return map[string]string{
		"User-Agent":   "Mozilla/5.0 BSGameSDK",
		"Content-Type": "application/x-www-form-urlencoded",
		"Host":         providerHost(),
	}
}

func rsaCreate(message, publicKey string) (string, error) {
	block, _ := pem.Decode([]byte(publicKey))
	var der []byte
	if block != nil {
		der = block.Bytes
	} else {
		der = []byte(publicKey)
	}

	var pub *rsa.PublicKey
	if parsed, err := x509.ParsePKIXPublicKey(der); err == nil {
		if key, ok := parsed.(*rsa.PublicKey); ok {
			pub = key
		}
	}
	if pub == nil {
		key, err := x509.ParsePKCS1PublicKey(der)
		if err != nil {
			return "", fmt.Errorf("parse rsa public key: %w", err)
		}
		pub = key
	}

	cipherText, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(message))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func pythonQuote(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "%2F", "/")
	return escaped
}

func (p payloadTemplate) SetSign(signatureMaterial string) string {
	now := int(time.Now().Unix())
	p.data["timestamp"] = now
	p.data["client_timestamp"] = now

	var body strings.Builder
	for _, key := range p.order {
		value, ok := p.data[key]
		if !ok {
			continue
		}
		if key == "pwd" {
			body.WriteString(key)
			body.WriteString("=")
			body.WriteString(pythonQuote(config.StringValue(value)))
			body.WriteString("&")
			continue
		}
		body.WriteString(key)
		body.WriteString("=")
		body.WriteString(config.StringValue(value))
		body.WriteString("&")
	}

	signKeys := make([]string, 0, len(p.data))
	for key := range p.data {
		signKeys = append(signKeys, key)
	}
	sort.Strings(signKeys)

	var signBase strings.Builder
	for _, key := range signKeys {
		signBase.WriteString(config.StringValue(p.data[key]))
	}
	sum := md5.Sum([]byte(signBase.String() + signatureMaterial))
	body.WriteString("sign=")
	body.WriteString(hex.EncodeToString(sum[:]))
	return body.String()
}

func providerHost() string {
	return strings.Join([]string{"line1-sdk-center-login-sh", ".biligame.net"}, "")
}

func providerBaseURL() string {
	return strings.Join([]string{"https://", providerHost(), "/"}, "")
}

func signatureMaterial() string {
	return strings.Join([]string{"dbf8f1b4496f430b", "8a3c0f436a35b931"}, "")
}

func (c *Client) effectiveProfile() effectiveSDKProfile {
	c.mu.RLock()
	runtime := c.runtimeProfile
	device := c.deviceProfile
	c.mu.RUnlock()
	return buildEffectiveSDKProfile(runtime, device)
}
