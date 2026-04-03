package buildinfo

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

var AppVersion = "1.1.3"
var BuildStamp = ""
var BuildFingerprint = ""

var (
	mu                      sync.Mutex
	runtimeBuildStamp       string
	runtimeBuildFingerprint string
)

func EffectiveBuildStamp() string {
	mu.Lock()
	defer mu.Unlock()

	if stamp := strings.TrimSpace(BuildStamp); stamp != "" {
		return stamp
	}
	if runtimeBuildStamp == "" {
		runtimeBuildStamp = "dev+" + randomHex(4) + "+" + time.Now().Format("060102150405")
	}
	return runtimeBuildStamp
}

func EffectiveBuildFingerprint() string {
	mu.Lock()
	defer mu.Unlock()

	if fp := normalizeFingerprint(BuildFingerprint); fp != "" {
		return fp
	}
	stamp := strings.TrimSpace(BuildStamp)
	if stamp != "" {
		sum := sha256.Sum256([]byte(strings.TrimSpace(AppVersion) + "|" + stamp))
		return hex.EncodeToString(sum[:16])
	}
	if runtimeBuildFingerprint == "" {
		runtimeBuildFingerprint = randomHex(16)
	}
	return runtimeBuildFingerprint
}

func normalizeFingerprint(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func randomHex(size int) string {
	if size <= 0 {
		size = 4
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "dev"
	}
	return hex.EncodeToString(buf)
}
