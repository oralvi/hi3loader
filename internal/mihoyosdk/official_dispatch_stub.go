//go:build !private_impl

package mihoyosdk

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"hi3loader/internal/config"
)

const (
	defaultDispatchURL   = ""
	officialDispatchName = "preferred_dispatch"
	customDispatchName   = "configured_dispatch"
)

func buildOfficialDispatchURL(_, _, _, _ string) (string, error) {
	return "", fmt.Errorf("preferred dispatch private implementation is unavailable in this build")
}

func isOfficialDispatchURL(baseURL string) bool {
	return strings.TrimSpace(baseURL) == ""
}

func UsesPrivateDispatch(baseURL string) bool {
	return isOfficialDispatchURL(baseURL)
}

func parseDispatchResponse(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("dispatch response empty")
	}

	resp := map[string]any{}
	if err := json.Unmarshal([]byte(trimmed), &resp); err == nil && len(resp) > 0 {
		return resp, nil
	}

	return map[string]any{
		"retcode": 0,
		"message": "OK",
		"data":    trimmed,
		"format":  "text",
	}, nil
}

func dispatchBlobSummary(data string) map[string]any {
	summary := map[string]any{}
	data = strings.TrimSpace(data)
	if data == "" {
		return summary
	}

	rawHash := sha256.Sum256([]byte(data))
	summary["raw_len"] = len(data)
	summary["raw_sha256"] = hex.EncodeToString(rawHash[:])

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		summary["decoded_error"] = err.Error()
		return summary
	}

	decodedHash := sha256.Sum256(decoded)
	summary["decoded_len"] = len(decoded)
	summary["decoded_sha256"] = hex.EncodeToString(decodedHash[:])
	return summary
}

func LooksLikeFinalDispatch(data string) bool {
	data = strings.TrimSpace(data)
	if data == "" {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return false
	}

	return len(decoded) >= 512
}

func addDispatchBlobSummary(resp map[string]any) {
	if resp == nil {
		return
	}
	data := config.StringValue(resp["data"])
	if data == "" {
		return
	}
	resp["blob_summary"] = dispatchBlobSummary(data)
}

func ShouldSkipPreferredDispatchCacheSource(source string) bool {
	source = strings.TrimSpace(strings.ToLower(source))
	return source == "" || source == "reference_third_party" || source == customDispatchName
}
