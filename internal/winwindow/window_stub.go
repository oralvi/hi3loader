//go:build !windows

package winwindow

import (
	"fmt"
	"image"
	"regexp"
)

type Window struct {
	Title       string
	Bounds      image.Rectangle
	ProcessID   uint32
	ProcessName string
}

func List() ([]Window, error) {
	return nil, fmt.Errorf("window enumeration is only supported on Windows")
}

func FindFirst(_ *regexp.Regexp, _ ...string) (*Window, error) {
	return nil, fmt.Errorf("window enumeration is only supported on Windows")
}

func Capture(_ *Window) (image.Image, error) {
	return nil, fmt.Errorf("window capture is only supported on Windows")
}

func IsForeground(_ *Window) bool {
	return false
}

func TitleMatches(actual, target string) bool {
	return actual == target
}

func TitleMatchesPattern(actual string, pattern *regexp.Regexp) bool {
	return pattern != nil && pattern.MatchString(actual)
}
