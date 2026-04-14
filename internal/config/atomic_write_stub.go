//go:build !windows

package config

import (
	"io/fs"
	"os"
)

func AtomicWriteFile(path string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(path, data, perm)
}
