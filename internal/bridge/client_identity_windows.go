package bridge

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func protectClientIdentity(plaintext []byte) ([]byte, error) {
	in := bytesToDataBlob(plaintext)
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, err
	}
	defer freeDataBlob(&out)
	return cloneDataBlob(&out), nil
}

func unprotectClientIdentity(ciphertext []byte) ([]byte, error) {
	in := bytesToDataBlob(ciphertext)
	var (
		name *uint16
		out  windows.DataBlob
	)
	if err := windows.CryptUnprotectData(&in, &name, nil, 0, nil, 0, &out); err != nil {
		return nil, err
	}
	defer freeDataBlob(&out)
	if name != nil {
		if _, err := windows.LocalFree(windows.Handle(unsafe.Pointer(name))); err != nil {
			return nil, fmt.Errorf("free protected name: %w", err)
		}
	}
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
