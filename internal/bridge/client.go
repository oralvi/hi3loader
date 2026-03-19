package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Client struct {
	executable string
	token      string
}

func NewClient(executable string) (*Client, error) {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return nil, nil
	}
	token, err := NewSessionToken()
	if err != nil {
		return nil, err
	}
	return &Client{executable: executable, token: token}, nil
}

func (c *Client) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	resp, err := c.invoke(ctx, Request{Op: opLogin, Login: &req})
	if err != nil {
		return LoginResponse{}, err
	}
	if resp.Login == nil {
		return LoginResponse{}, fmt.Errorf("helper login response missing payload")
	}
	return *resp.Login, nil
}

func (c *Client) VerifySession(ctx context.Context, req VerifyRequest) (VerifyResponse, error) {
	resp, err := c.invoke(ctx, Request{Op: opVerifySession, Verify: &req})
	if err != nil {
		return VerifyResponse{}, err
	}
	if resp.Verify == nil {
		return VerifyResponse{}, fmt.Errorf("helper verify response missing payload")
	}
	return *resp.Verify, nil
}

func (c *Client) FetchReleaseInfo(ctx context.Context) (ReleaseInfoResponse, error) {
	resp, err := c.invoke(ctx, Request{Op: opFetchReleaseInfo})
	if err != nil {
		return ReleaseInfoResponse{}, err
	}
	if resp.ReleaseInfo == nil {
		return ReleaseInfoResponse{}, fmt.Errorf("helper release response missing payload")
	}
	return *resp.ReleaseInfo, nil
}

func (c *Client) FetchCredential(ctx context.Context) (CredentialResponse, error) {
	resp, err := c.invoke(ctx, Request{Op: opFetchCredential})
	if err != nil {
		return CredentialResponse{}, err
	}
	if resp.Credential == nil {
		return CredentialResponse{}, fmt.Errorf("helper credential response missing payload")
	}
	return *resp.Credential, nil
}

func (c *Client) ResolveDispatch(ctx context.Context, req DispatchRequest) (DispatchResponse, error) {
	resp, err := c.invoke(ctx, Request{Op: opResolveDispatch, Dispatch: &req})
	if err != nil {
		return DispatchResponse{}, err
	}
	if resp.Dispatch == nil {
		return DispatchResponse{}, fmt.Errorf("helper dispatch response missing payload")
	}
	return *resp.Dispatch, nil
}

func (c *Client) ScanCheck(ctx context.Context, req ScanRequest) (ScanResponse, error) {
	resp, err := c.invoke(ctx, Request{Op: opScanCheck, Scan: &req})
	if err != nil {
		return ScanResponse{}, err
	}
	if resp.Scan == nil {
		return ScanResponse{}, fmt.Errorf("helper scan response missing payload")
	}
	return *resp.Scan, nil
}

func (c *Client) invoke(ctx context.Context, req Request) (Response, error) {
	if c == nil || strings.TrimSpace(c.executable) == "" || strings.TrimSpace(c.token) == "" {
		return Response{}, fmt.Errorf("helper executable is unavailable")
	}

	cmd := exec.CommandContext(ctx, c.executable, helperArg)
	configureCommand(cmd)
	authFile, err := writeHelperAuthFile(c.token)
	if err != nil {
		return Response{}, err
	}
	defer os.Remove(authFile)
	cmd.Env = append(
		os.Environ(),
		helperTokenEnv+"="+c.token,
		helperAuthFileEnv+"="+authFile,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return Response{}, fmt.Errorf("helper stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Response{}, fmt.Errorf("helper stdout: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return Response{}, fmt.Errorf("start helper: %w", err)
	}

	encodeErr := json.NewEncoder(stdin).Encode(req)
	_ = stdin.Close()
	if encodeErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Response{}, fmt.Errorf("encode helper request: %w", encodeErr)
	}

	var resp Response
	decodeErr := json.NewDecoder(stdout).Decode(&resp)
	waitErr := cmd.Wait()
	if decodeErr != nil {
		if stderr.Len() > 0 {
			return Response{}, fmt.Errorf("decode helper response: %w (%s)", decodeErr, strings.TrimSpace(stderr.String()))
		}
		return Response{}, fmt.Errorf("decode helper response: %w", decodeErr)
	}
	if waitErr != nil {
		if stderr.Len() > 0 {
			return Response{}, fmt.Errorf("helper exited: %w (%s)", waitErr, strings.TrimSpace(stderr.String()))
		}
		return Response{}, fmt.Errorf("helper exited: %w", waitErr)
	}
	if resp.Error != "" {
		return Response{}, fmt.Errorf("%s", strings.TrimSpace(resp.Error))
	}
	return resp, nil
}

func writeHelperAuthFile(token string) (string, error) {
	tmp, err := os.CreateTemp("", filepath.Base(helperArg)+"-auth-*")
	if err != nil {
		return "", fmt.Errorf("create helper authorization file: %w", err)
	}
	path := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(path)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("chmod helper authorization file: %w", err)
	}
	if _, err := tmp.WriteString(token); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write helper authorization file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("sync helper authorization file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close helper authorization file: %w", err)
	}

	cleanup = false
	return path, nil
}
