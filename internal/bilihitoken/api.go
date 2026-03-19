package bilihitoken

import "errors"

var ErrProviderUnavailable = errors.New("local credential provider is unavailable in this build")

type ReleaseInfo struct {
	PackageURL string
	Version    int
	BHVer      string
}

func IsProviderUnavailable(err error) bool {
	return errors.Is(err, ErrProviderUnavailable)
}
