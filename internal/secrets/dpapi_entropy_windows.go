//go:build windows

package secrets

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const (
	dpapiEntropySalt    = "hi3loader.dpapi.entropy.v1"
	dpapiEntropyDirName = "HI3Loader"
)

func loadOrCreateDPAPIEntropy() ([]byte, error) {
	path, seed, err := dpapiEntropyFile()
	if err != nil {
		return nil, err
	}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if aclErr := restrictEntropyFileACL(path); aclErr != nil {
			return nil, aclErr
		}
		return data, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create dpapi entropy dir: %w", err)
	}
	if err := os.WriteFile(path, seed, 0o600); err != nil {
		return nil, fmt.Errorf("write dpapi entropy file: %w", err)
	}
	if err := restrictEntropyFileACL(path); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return append([]byte(nil), seed...), nil
}

func dpapiEntropyFile() (string, []byte, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", nil, fmt.Errorf("resolve executable path: %w", err)
	}
	normalizedExePath := strings.ToLower(filepath.Clean(exePath))
	pathHash := sha256.Sum256([]byte(normalizedExePath))
	entropy := sha256.Sum256([]byte(dpapiEntropySalt + "\n" + normalizedExePath))

	root := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if root == "" {
		root, err = os.UserConfigDir()
		if err != nil {
			return "", nil, fmt.Errorf("resolve dpapi entropy root: %w", err)
		}
	}
	dir := filepath.Join(root, dpapiEntropyDirName, "secrets")
	fileName := "dpapi-entropy-" + hex.EncodeToString(pathHash[:8]) + ".bin"
	return filepath.Join(dir, fileName), append([]byte(nil), entropy[:]...), nil
}

func restrictEntropyFileACL(path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("read current token user: %w", err)
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return fmt.Errorf("resolve local system sid: %w", err)
	}

	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.GRANT_ACCESS,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_USER,
				TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
			},
		},
		{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.GRANT_ACCESS,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_USER,
				TrusteeValue: windows.TrusteeValueFromSID(systemSID),
			},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("build entropy acl: %w", err)
	}

	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	); err != nil {
		return fmt.Errorf("apply entropy acl: %w", err)
	}
	return nil
}
