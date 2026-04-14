//go:build !windows

package winwindow

import (
	"errors"
	"image"
	"regexp"
)

var ErrTargetWindowNotFound = errors.New("target window not found")

type Window struct {
	Handle      uintptr
	Title       string
	Bounds      image.Rectangle
	ProcessID   uint32
	ProcessName string
}

func List() ([]Window, error) {
	return nil, nil
}

func FindFirst(_ *regexp.Regexp, _ ...string) (*Window, error) {
	return nil, ErrTargetWindowNotFound
}

func Capture(_ *Window) (image.Image, error) {
	return nil, errors.New("not supported on this platform")
}
