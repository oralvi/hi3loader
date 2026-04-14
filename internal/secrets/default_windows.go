//go:build windows

package secrets

func NewDefaultStore() (SecretStore, error) {
	return NewDPAPIStore(), nil
}
