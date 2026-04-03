package bridge

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"hi3loader/internal/loaderapiv1"
)

type serverAuthority struct {
	KeyID     string
	PublicKey []byte
}

var embeddedServerAuthority serverAuthority

func HasEmbeddedServerAuthority() bool {
	return strings.TrimSpace(embeddedServerAuthority.KeyID) != "" && len(embeddedServerAuthority.PublicKey) == ed25519.PublicKeySize
}

func normalizeServerCertFingerprint(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func fingerprintMatchesCertificate(expected string, certDER []byte) bool {
	expected = normalizeServerCertFingerprint(expected)
	if expected == "" || len(certDER) == 0 {
		return false
	}
	sum := sha256.Sum256(certDER)
	return hex.EncodeToString(sum[:]) == expected
}

func manifestSignaturePayload(keyID, tlsCertSHA256, protocolVersion, serverName, generatedAt string) []byte {
	parts := []string{
		strings.TrimSpace(keyID),
		normalizeServerCertFingerprint(tlsCertSHA256),
		strings.TrimSpace(protocolVersion),
		strings.TrimSpace(serverName),
		strings.TrimSpace(generatedAt),
	}
	return []byte(strings.Join(parts, "\n"))
}

func verifySignedManifest(resp *loaderapiv1.ManifestResponse) error {
	if !HasEmbeddedServerAuthority() {
		return errMissingEmbeddedAuthority
	}
	if resp == nil {
		return errInvalidManifest("missing manifest response")
	}
	if strings.TrimSpace(resp.KeyId) == "" {
		return errInvalidManifest("missing manifest key id")
	}
	if strings.TrimSpace(resp.ProtocolVersion) == "" {
		return errInvalidManifest("missing manifest protocol version")
	}
	if strings.TrimSpace(resp.TlsCertSha256) == "" {
		return errInvalidManifest("missing manifest tls fingerprint")
	}
	if len(resp.Signature) == 0 {
		return errInvalidManifest("missing manifest signature")
	}
	payload := manifestSignaturePayload(resp.KeyId, resp.TlsCertSha256, resp.ProtocolVersion, resp.ServerName, resp.GeneratedAt)
	if !ed25519.Verify(ed25519.PublicKey(embeddedServerAuthority.PublicKey), payload, resp.Signature) {
		return errInvalidManifest("invalid manifest signature")
	}
	if strings.TrimSpace(resp.ProtocolVersion) != loaderapiv1.ProtocolValue {
		return errInvalidManifest("protocol mismatch")
	}
	return nil
}

type manifestValidationError string

func (e manifestValidationError) Error() string { return string(e) }

func errInvalidManifest(message string) error {
	return manifestValidationError(strings.TrimSpace(message))
}

var errMissingEmbeddedAuthority = manifestValidationError("missing embedded server authority")

func SetEmbeddedServerAuthorityForTesting(publicKey []byte) func() {
	old := embeddedServerAuthority
	embeddedServerAuthority = serverAuthority{
		KeyID:     "test",
		PublicKey: append([]byte(nil), publicKey...),
	}
	return func() {
		embeddedServerAuthority = old
	}
}
