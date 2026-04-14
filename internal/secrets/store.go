package secrets

type SecretStore interface {
	Protect(plaintext []byte) ([]byte, error)
	Unprotect(ciphertext []byte) ([]byte, error)
	Close() error
}

func wipeBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}
