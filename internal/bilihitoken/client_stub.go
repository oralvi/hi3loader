//go:build !private_impl

package bilihitoken

import (
	"fmt"
	"net/http"
)

func FetchReleaseInfo(_ *http.Client) (*ReleaseInfo, error) {
	return nil, ErrProviderUnavailable
}

func FetchCredential(_ *http.Client, _ string) (string, error) {
	return "", fmt.Errorf("fetch provider credential: %w", ErrProviderUnavailable)
}
