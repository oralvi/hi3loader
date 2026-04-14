//go:build windows

package secrets

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type DPAPIStore struct {
	once       sync.Once
	entropy    []byte
	entropyErr error
}

func NewDPAPIStore() *DPAPIStore {
	return &DPAPIStore{}
}

func (s *DPAPIStore) Protect(plaintext []byte) ([]byte, error) {
	entropy, err := s.entropyBytes()
	if err != nil {
		return nil, err
	}
	in := bytesToDataBlob(plaintext)
	entropyBlob := bytesToDataBlob(entropy)
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, &entropyBlob, 0, nil, 0, &out); err != nil {
		return nil, err
	}
	defer freeDataBlob(&out)
	return cloneDataBlob(&out), nil
}

func (s *DPAPIStore) Unprotect(ciphertext []byte) ([]byte, error) {
	entropy, err := s.entropyBytes()
	if err != nil {
		return nil, err
	}
	in := bytesToDataBlob(ciphertext)
	entropyBlob := bytesToDataBlob(entropy)
	var (
		name *uint16
		out  windows.DataBlob
	)
	if err := windows.CryptUnprotectData(&in, &name, &entropyBlob, 0, nil, 0, &out); err != nil {
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

func (s *DPAPIStore) Close() error {
	if s == nil {
		return nil
	}
	wipeBytes(s.entropy)
	s.entropy = nil
	s.entropyErr = nil
	return nil
}

func (s *DPAPIStore) entropyBytes() ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("dpapi store is nil")
	}
	s.once.Do(func() {
		s.entropy, s.entropyErr = loadOrCreateDPAPIEntropy()
	})
	if s.entropyErr != nil {
		return nil, s.entropyErr
	}
	return s.entropy, nil
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
