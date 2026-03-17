package bsgamesdk

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"hi3loader/internal/config"
	"hi3loader/internal/netutil"
)

const biliLoginBase = "https://line1-sdk-center-login-sh.biligame.net/"

type Client struct {
	http *http.Client
}

type payloadTemplate struct {
	raw   string
	data  map[string]any
	order []string
}

func NewClient() *Client {
	return &Client{http: netutil.NewClient()}
}

func (c *Client) GetUserInfo(ctx context.Context, uid, accessKey string) (map[string]any, error) {
	payload := userInfoTemplate.Clone()
	payload.data["uid"] = uid
	payload.data["access_key"] = accessKey
	body := payload.SetSign()

	resp := map[string]any{}
	err := netutil.PostBodyJSON(ctx, c.http, biliLoginBase+"/api/client/user.info", body, defaultHeaders(), &resp)
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
	payload := captchaTemplate.Clone()
	body := payload.SetSign()

	resp := map[string]any{}
	err := netutil.PostBodyJSON(ctx, c.http, biliLoginBase+"api/client/start_captcha", body, defaultHeaders(), &resp)
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
	rsaResp, err := c.requestRSA(ctx)
	if err != nil {
		return nil, err
	}

	payload := loginTemplate.Clone()
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
	body := payload.SetSign()

	resp := map[string]any{}
	err = netutil.PostBodyJSON(ctx, c.http, biliLoginBase+"api/client/login", body, defaultHeaders(), &resp)
	return resp, err
}

func (c *Client) loginWithCaptcha(ctx context.Context, account, password string, cap map[string]any) (map[string]any, error) {
	rsaResp, err := c.requestRSA(ctx)
	if err != nil {
		return nil, err
	}

	payload := loginTemplate.Clone()
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
	body := payload.SetSign()

	resp := map[string]any{}
	err = netutil.PostBodyJSON(ctx, c.http, biliLoginBase+"api/client/login", body, defaultHeaders(), &resp)
	return resp, err
}

func (c *Client) requestRSA(ctx context.Context) (map[string]any, error) {
	payload := rsaTemplate.Clone()
	body := payload.SetSign()

	resp := map[string]any{}
	err := netutil.PostBodyJSON(ctx, c.http, biliLoginBase+"api/client/rsa", body, defaultHeaders(), &resp)
	return resp, err
}

func defaultHeaders() map[string]string {
	return map[string]string{
		"User-Agent":   "Mozilla/5.0 BSGameSDK",
		"Content-Type": "application/x-www-form-urlencoded",
		"Host":         "line1-sdk-center-login-sh.biligame.net",
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

func newPayloadTemplate(raw string) payloadTemplate {
	data := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		panic(err)
	}

	re := regexp.MustCompile(`"([^"]+)":`)
	matches := re.FindAllStringSubmatch(raw, -1)
	order := make([]string, 0, len(matches))
	seen := map[string]struct{}{}
	for _, match := range matches {
		key := match[1]
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		order = append(order, key)
	}

	return payloadTemplate{
		raw:   raw,
		data:  data,
		order: order,
	}
}

func (p payloadTemplate) Clone() payloadTemplate {
	clone := payloadTemplate{
		raw:   p.raw,
		data:  make(map[string]any, len(p.data)),
		order: append([]string(nil), p.order...),
	}
	for k, v := range p.data {
		clone.data[k] = v
	}
	return clone
}

func (p payloadTemplate) SetSign() string {
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
	sum := md5.Sum([]byte(signBase.String() + "dbf8f1b4496f430b8a3c0f436a35b931"))
	body.WriteString("sign=")
	body.WriteString(hex.EncodeToString(sum[:]))
	return body.String()
}

var userInfoTemplate = newPayloadTemplate(`{"cur_buvid":"XZA2FA4AC240F665E2F27F603ABF98C615C29","client_timestamp":"1667057013442","sdk_type":"1","isRoot":"0","merchant_id":"590","dp":"1280*720","mac":"08:00:27:53:DD:12","uid":"437470182","support_abis":"x86,armeabi-v7a,armeabi","apk_sign":"4502a02a00395dec05a4134ad593224d","platform_type":"3","old_buvid":"XZA2FA4AC240F665E2F27F603ABF98C615C29","operators":"5","fingerprint":"","model":"MuMu","udid":"XXA31CBAB6CBA63E432E087B58411A213BFB7","net":"5","app_id":"180","brand":"Android","oaid":"","game_id":"180","timestamp":"1667057013275","ver":"6.1.0","c":"1","version_code":"510","server_id":"378","version":"1","domain_switch_count":"0","pf_ver":"12","access_key":"","domain":"line1-sdk-center-login-sh.biligame.net","original_domain":"","imei":"","sdk_log_type":"1","sdk_ver":"3.4.2","android_id":"84567e2dda72d1d4","channel_id":1}`)

var rsaTemplate = newPayloadTemplate(`{"operators":"5","merchant_id":"590","isRoot":"0","domain_switch_count":"0","sdk_type":"1","sdk_log_type":"1","timestamp":"1613035485639","support_abis":"x86,armeabi-v7a,armeabi","access_key":"","sdk_ver":"3.4.2","oaid":"","dp":"1280*720","original_domain":"","imei":"","version":"1","udid":"KREhESMUIhUjFnJKNko2TDQFYlZkB3cdeQ==","apk_sign":"4502a02a00395dec05a4134ad593224d","platform_type":"3","old_buvid":"XZA2FA4AC240F665E2F27F603ABF98C615C29","android_id":"84567e2dda72d1d4","fingerprint":"","mac":"08:00:27:53:DD:12","server_id":"378","domain":"line1-sdk-center-login-sh.biligame.net","app_id":"180","version_code":"510","net":"4","pf_ver":"12","cur_buvid":"XZA2FA4AC240F665E2F27F603ABF98C615C29","c":"1","brand":"Android","client_timestamp":"1613035486888","channel_id":"1","uid":"","game_id":"180","ver":"6.1.0","model":"MuMu"} `)

var loginTemplate = newPayloadTemplate(`{"operators":"5","merchant_id":"590","isRoot":"0","domain_switch_count":"0","sdk_type":"1","sdk_log_type":"1","timestamp":"1613035508188","support_abis":"x86,armeabi-v7a,armeabi","access_key":"","sdk_ver":"3.4.2","oaid":"","dp":"1280*720","original_domain":"","imei":"227656364311444","gt_user_id":"fac83ce4326d47e1ac277a4d552bd2af","seccode":"","version":"1","udid":"KREhESMUIhUjFnJKNko2TDQFYlZkB3cdeQ==","apk_sign":"4502a02a00395dec05a4134ad593224d","platform_type":"3","old_buvid":"XZA2FA4AC240F665E2F27F603ABF98C615C29","android_id":"84567e2dda72d1d4","fingerprint":"","validate":"84ec07cff0d9c30acb9fe46b8745e8df","mac":"08:00:27:53:DD:12","server_id":"378","domain":"line1-sdk-center-login-sh.biligame.net","app_id":"180","pwd":"rxwA8J+GcVdqa3qlvXFppusRg4Ss83tH6HqxcciVsTdwxSpsoz2WuAFFGgQKWM1+GtFovrLkpeMieEwOmQdzvDiLTtHeQNBOiqHDfJEKtLj7h1nvKZ1Op6vOgs6hxM6fPqFGQC2ncbAR5NNkESpSWeYTO4IT58ZIJcC0DdWQqh4=","version_code":"510","net":"4","pf_ver":"12","cur_buvid":"XZA2FA4AC240F665E2F27F603ABF98C615C29","c":"1","brand":"Android","client_timestamp":"1613035509437","channel_id":"1","uid":"","captcha_type":"1","game_id":"180","challenge":"efc825eaaef2405c954a91ad9faf29a2","user_id":"doo349","ver":"6.1.0","model":"MuMu"} `)

var captchaTemplate = newPayloadTemplate(`{"operators":"5","merchant_id":"590","isRoot":"0","domain_switch_count":"0","sdk_type":"1","sdk_log_type":"1","timestamp":"1613035486182","support_abis":"x86,armeabi-v7a,armeabi","access_key":"","sdk_ver":"3.4.2","oaid":"","dp":"1280*720","original_domain":"","imei":"227656364311444","version":"1","udid":"KREhESMUIhUjFnJKNko2TDQFYlZkB3cdeQ==","apk_sign":"4502a02a00395dec05a4134ad593224d","platform_type":"3","old_buvid":"XZA2FA4AC240F665E2F27F603ABF98C615C29","android_id":"84567e2dda72d1d4","fingerprint":"","mac":"08:00:27:53:DD:12","server_id":"378","domain":"line1-sdk-center-login-sh.biligame.net","app_id":"180","version_code":"510","net":"4","pf_ver":"12","cur_buvid":"XZA2FA4AC240F665E2F27F603ABF98C615C29","c":"1","brand":"Android","client_timestamp":"1613035487431","channel_id":"1","uid":"","game_id":"180","ver":"6.1.0","model":"MuMu"} `)
