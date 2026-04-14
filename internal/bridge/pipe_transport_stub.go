//go:build !windows

package bridge

import (
	"context"
	"net"
	"time"
)

func newLoaderAPIDialContext(_ string, _ loaderAPIDeployMode, _ bool) func(context.Context, string, string) (net.Conn, error) {
	return (&net.Dialer{Timeout: 10 * time.Second}).DialContext
}
