//go:build windows

package bridge

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
)

const loaderAPIDefaultPipeBaseName = `\\.\pipe\rinki-hi3`

var (
	loaderAPIDialPipeContext  = winio.DialPipeContext
	loaderAPINewDefaultDialer = func() func(context.Context, string, string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext
	}
)

func newLoaderAPIDialContext(baseURL string, deployMode loaderAPIDeployMode, allowTCPFallback bool) func(context.Context, string, string) (net.Conn, error) {
	defaultDialer := loaderAPINewDefaultDialer()
	pipeName, ok := loaderAPIPipeNameForBaseURL(baseURL)
	if !ok {
		return defaultDialer
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := loaderAPIDialPipeContext(ctx, pipeName)
		if err == nil {
			return conn, nil
		}
		if deployMode == loaderAPIDeployLocal && !allowTCPFallback {
			return nil, err
		}
		if deployMode != loaderAPIDeployRemote && !allowTCPFallback {
			return nil, err
		}
		return defaultDialer(ctx, network, address)
	}
}

func loaderAPIPipeNameForBaseURL(baseURL string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	switch host {
	case "127.0.0.1", "localhost", "::1", "":
	default:
		return "", false
	}
	port := strings.TrimSpace(parsed.Port())
	if port == "" {
		return "", false
	}
	return loaderAPIDefaultPipeBaseName + "-" + port, true
}
