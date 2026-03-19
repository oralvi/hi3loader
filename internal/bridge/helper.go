package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"hi3loader/internal/bilihitoken"
	"hi3loader/internal/bsgamesdk"
	"hi3loader/internal/config"
	"hi3loader/internal/mihoyosdk"
	"hi3loader/internal/netutil"
)

func HandleAuxRuntime(args []string, in io.Reader, out io.Writer) (bool, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) != helperArg {
		return false, nil
	}
	if err := authorizeAuxRuntime(); err != nil {
		return true, err
	}
	return true, serveOnce(in, out)
}

func serveOnce(in io.Reader, out io.Writer) error {
	var req Request
	if err := json.NewDecoder(in).Decode(&req); err != nil {
		return fmt.Errorf("decode helper request: %w", err)
	}

	resp, err := handleRequest(context.Background(), req)
	if err != nil {
		resp.Error = err.Error()
	}
	if err := json.NewEncoder(out).Encode(resp); err != nil {
		return fmt.Errorf("encode helper response: %w", err)
	}
	return nil
}

func handleRequest(ctx context.Context, req Request) (Response, error) {
	switch req.Op {
	case opLogin:
		if req.Login == nil {
			return Response{}, fmt.Errorf("missing login request")
		}
		resp, err := handleLogin(ctx, *req.Login)
		return Response{Login: &resp}, err
	case opVerifySession:
		if req.Verify == nil {
			return Response{}, fmt.Errorf("missing verify request")
		}
		resp, err := handleVerify(ctx, *req.Verify)
		return Response{Verify: &resp}, err
	case opFetchReleaseInfo:
		resp, err := handleFetchReleaseInfo()
		return Response{ReleaseInfo: &resp}, err
	case opFetchCredential:
		resp, err := handleFetchCredential()
		return Response{Credential: &resp}, err
	case opResolveDispatch:
		if req.Dispatch == nil {
			return Response{}, fmt.Errorf("missing dispatch request")
		}
		resp, err := handleResolveDispatch(ctx, *req.Dispatch)
		return Response{Dispatch: &resp}, err
	case opScanCheck:
		if req.Scan == nil {
			return Response{}, fmt.Errorf("missing scan request")
		}
		resp, err := handleScanCheck(ctx, *req.Scan)
		return Response{Scan: &resp}, err
	default:
		return Response{}, fmt.Errorf("unknown helper op: %s", req.Op)
	}
}

func handleLogin(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	client := bsgamesdk.NewClient()
	mhy := mihoyosdk.NewClient()
	resp := LoginResponse{}

	uid := ""
	if req.UID != 0 {
		uid = fmt.Sprintf("%d", req.UID)
	}
	accessKey := strings.TrimSpace(req.AccessKey)
	if req.LastLoginSucc && req.UID != 0 && accessKey != "" {
		info, err := client.GetUserInfo(ctx, uid, accessKey)
		if err == nil && strings.TrimSpace(config.StringValue(info["uname"])) != "" {
			resp.UName = strings.TrimSpace(config.StringValue(info["uname"]))
		} else {
			uid = ""
			accessKey = ""
		}
	}

	if accessKey == "" {
		account := strings.TrimSpace(req.Account)
		password := req.Password
		if account == "" || password == "" {
			return resp, fmt.Errorf("account and password are required")
		}

		var captcha map[string]any
		if req.Captcha != nil {
			captcha = map[string]any{
				"challenge": req.Captcha.Challenge,
				"validate":  req.Captcha.Validate,
				"userid":    req.Captcha.UserID,
			}
		}

		loginResp, err := client.Login(ctx, account, password, captcha)
		if err != nil {
			return resp, err
		}
		accessKey = strings.TrimSpace(config.StringValue(loginResp["access_key"]))
		resp.Message = strings.TrimSpace(config.StringValue(loginResp["message"]))
		if accessKey == "" {
			capData, err := client.StartCaptcha(ctx)
			if err == nil {
				resp.NeedsCaptcha = true
				resp.CaptchaGT = strings.TrimSpace(config.StringValue(capData["gt"]))
				resp.CaptchaChallenge = strings.TrimSpace(config.StringValue(capData["challenge"]))
				resp.CaptchaUserID = strings.TrimSpace(config.StringValue(capData["gt_user_id"]))
			}
			return resp, nil
		}

		uid = strings.TrimSpace(config.StringValue(loginResp["uid"]))
		if uid == "" {
			return resp, fmt.Errorf("bilibili login response missing uid")
		}
		info, err := client.GetUserInfo(ctx, uid, accessKey)
		if err != nil {
			return resp, err
		}
		resp.UName = strings.TrimSpace(config.StringValue(info["uname"]))
		if resp.UName == "" {
			return resp, fmt.Errorf("bilibili user info missing uname")
		}
	}

	verifyResp, err := mhy.Verify(ctx, uid, accessKey)
	if err != nil {
		return resp, err
	}
	resp.VerifyRetcode = config.Int64Value(verifyResp["retcode"])
	resp.Message = strings.TrimSpace(config.StringValue(verifyResp["message"]))
	if resp.VerifyRetcode != 0 {
		resp.UID = config.Int64Value(uid)
		resp.AccessKey = accessKey
		return resp, nil
	}

	session, err := mihoyosdk.ExtractSessionInfo(verifyResp)
	if err != nil {
		return resp, err
	}

	resp.UID = config.Int64Value(uid)
	resp.AccessKey = accessKey
	resp.Session = *session
	return resp, nil
}

func handleVerify(ctx context.Context, req VerifyRequest) (VerifyResponse, error) {
	resp := VerifyResponse{}
	uid := ""
	if req.UID != 0 {
		uid = fmt.Sprintf("%d", req.UID)
	}
	verifyResp, err := mihoyosdk.NewClient().Verify(ctx, uid, strings.TrimSpace(req.AccessKey))
	if err != nil {
		return resp, err
	}
	resp.Retcode = config.Int64Value(verifyResp["retcode"])
	resp.Message = strings.TrimSpace(config.StringValue(verifyResp["message"]))
	if resp.Retcode != 0 {
		return resp, nil
	}
	session, err := mihoyosdk.ExtractSessionInfo(verifyResp)
	if err != nil {
		return resp, err
	}
	resp.Session = *session
	return resp, nil
}

func handleFetchReleaseInfo() (ReleaseInfoResponse, error) {
	info, err := bilihitoken.FetchReleaseInfo(netutil.NewClient())
	if err != nil {
		return ReleaseInfoResponse{}, err
	}
	return ReleaseInfoResponse{
		Version: info.Version,
		BHVer:   strings.TrimSpace(info.BHVer),
	}, nil
}

func handleFetchCredential() (CredentialResponse, error) {
	info, err := bilihitoken.FetchReleaseInfo(netutil.NewClient())
	if err != nil {
		return CredentialResponse{}, err
	}
	token, err := bilihitoken.FetchCredential(netutil.NewClient(), info.PackageURL)
	if err != nil {
		return CredentialResponse{}, err
	}
	return CredentialResponse{
		Token:   strings.TrimSpace(token),
		Version: info.Version,
		BHVer:   strings.TrimSpace(info.BHVer),
	}, nil
}

func handleResolveDispatch(ctx context.Context, req DispatchRequest) (DispatchResponse, error) {
	cfg := req.Config.ToConfig()
	respMap, err := mihoyosdk.NewClient().GetOAServer(ctx, strings.TrimSpace(req.UID), cfg)
	if err != nil {
		return DispatchResponse{}, err
	}
	return dispatchResponseFromMap(respMap), nil
}

func handleScanCheck(ctx context.Context, req ScanRequest) (ScanResponse, error) {
	cfg := req.Config.ToConfig()
	respMap, err := mihoyosdk.NewClient().ScanCheck(ctx, req.Session, strings.TrimSpace(req.Ticket), cfg)
	if err != nil {
		return ScanResponse{}, err
	}
	return ScanResponse{
		Retcode: config.Int64Value(respMap["retcode"]),
		Message: strings.TrimSpace(config.StringValue(respMap["message"])),
	}, nil
}

func dispatchResponseFromMap(respMap map[string]any) DispatchResponse {
	resp := DispatchResponse{
		Retcode: config.Int64Value(respMap["retcode"]),
		Message: strings.TrimSpace(config.StringValue(respMap["message"])),
		Data:    strings.TrimSpace(config.StringValue(respMap["data"])),
		Source:  strings.TrimSpace(config.StringValue(respMap["source"])),
	}
	resp.CachedSource = strings.TrimSpace(config.StringValue(respMap["cached_source"]))
	resp.CachedSavedAt = strings.TrimSpace(config.StringValue(respMap["cached_saved_at"]))
	if summary, ok := respMap["blob_summary"].(map[string]any); ok {
		resp.BlobSummary = BlobSummary{
			RawLen:        int(config.Int64Value(summary["raw_len"])),
			DecodedLen:    int(config.Int64Value(summary["decoded_len"])),
			DecodedSHA256: strings.TrimSpace(config.StringValue(summary["decoded_sha256"])),
		}
	}
	return resp
}
