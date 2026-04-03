//go:build windows

package config

import (
	"encoding/base64"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func protectSessionSecrets(plaintext []byte) (string, error) {
	in := bytesToDataBlob(plaintext)
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return "", err
	}
	defer freeDataBlob(&out)
	return base64.StdEncoding.EncodeToString(cloneDataBlob(&out)), nil
}

func unprotectSessionSecrets(ciphertext string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode session blob: %w", err)
	}
	in := bytesToDataBlob(raw)
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, err
	}
	defer freeDataBlob(&out)
	return cloneDataBlob(&out), nil
}

func bytesToDataBlob(data []byte) windows.DataBlob {
	if len(data) == 0 {
		return windows.DataBlob{}
	}
	return windows.DataBlob{
		Size: uint32(len(data)),
		Data: &data[0],
	}
}

func cloneDataBlob(blob *windows.DataBlob) []byte {
	if blob == nil || blob.Data == nil || blob.Size == 0 {
		return nil
	}
	raw := unsafe.Slice(blob.Data, blob.Size)
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}

func freeDataBlob(blob *windows.DataBlob) {
	if blob == nil || blob.Data == nil {
		return
	}
	_, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(blob.Data)))
	blob.Data = nil
	blob.Size = 0
}
