package bridge

import (
	"context"
	"fmt"
	"strings"
)

func ExecuteScan(ctx context.Context, req ScanRequest) (ScanResponse, error) {
	if err := validateRemoteScanRequest(req); err != nil {
		return ScanResponse{}, err
	}
	resp, err := newLoaderAPIClient(req.LoaderAPIURL).ExecuteScan(ctx, req)
	if err != nil {
		return ScanResponse{}, err
	}
	return ScanResponse{
		Retcode: resp.Retcode,
		Message: strings.TrimSpace(resp.Message),
	}, nil
}

func validateRemoteScanRequest(req ScanRequest) error {
	if err := validateClientMeta(req.ClientMeta); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(req.Ticket) == "":
		return fmt.Errorf("missing ticket")
	case strings.TrimSpace(req.AccessKey) == "":
		return fmt.Errorf("missing access_key")
	case strings.TrimSpace(req.LoaderAPIURL) == "":
		return fmt.Errorf("missing loader_api_url")
	default:
		return nil
	}
}

func validateClientMeta(meta ClientMeta) error {
	switch {
	case strings.TrimSpace(meta.ClientName) == "":
		return fmt.Errorf("missing client_meta.client_name")
	case strings.TrimSpace(meta.ClientVersion) == "":
		return fmt.Errorf("missing client_meta.client_version")
	case strings.TrimSpace(meta.BuildFingerprint) == "":
		return fmt.Errorf("missing client_meta.build_fingerprint")
	case strings.TrimSpace(meta.Platform) == "":
		return fmt.Errorf("missing client_meta.platform")
	case strings.TrimSpace(meta.Locale) == "":
		return fmt.Errorf("missing client_meta.locale")
	case strings.TrimSpace(meta.TransportMode) == "":
		return fmt.Errorf("missing client_meta.transport_mode")
	default:
		return nil
	}
}
