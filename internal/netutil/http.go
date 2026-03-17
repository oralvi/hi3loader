package netutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

func NewClient() *http.Client {
	transport := &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 15 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
	}

	return &http.Client{
		Timeout:   20 * time.Second,
		Transport: transport,
	}
}

func GetJSON(ctx context.Context, client *http.Client, target string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http status %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func GetText(ctx context.Context, client *http.Client, target string, headers map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func PostBodyJSON(ctx context.Context, client *http.Client, target string, body string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http status %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func PostEmpty(ctx context.Context, client *http.Client, target string, body string, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http status %s", resp.Status)
	}
	return nil
}
