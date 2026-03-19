package bridge

import (
	"crypto/subtle"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func authorizeHelperSecret() error {
	token := strings.TrimSpace(os.Getenv(helperTokenEnv))
	authPath := strings.TrimSpace(os.Getenv(helperAuthFileEnv))
	if token == "" || authPath == "" {
		return fmt.Errorf("helper authorization is missing")
	}

	data, err := os.ReadFile(authPath)
	_ = os.Remove(authPath)
	if err != nil {
		return fmt.Errorf("helper authorization file is unavailable")
	}

	authToken := strings.TrimSpace(string(data))
	if subtle.ConstantTimeCompare([]byte(authToken), []byte(token)) != 1 {
		return fmt.Errorf("helper authorization is invalid")
	}
	return nil
}

func sameExecutablePath(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	leftAbs, _ := filepath.Abs(left)
	rightAbs, _ := filepath.Abs(right)
	return strings.EqualFold(leftAbs, rightAbs)
}
