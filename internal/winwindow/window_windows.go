//go:build windows

package winwindow

import (
	"errors"
	"fmt"
	"image"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
	xwindows "golang.org/x/sys/windows"
)

var (
	ErrTargetWindowNotFound = errors.New("target window not found")

	user32                  = syscall.NewLazyDLL("user32.dll")
	procEnumWindows         = user32.NewProc("EnumWindows")
	procGetWindowDC         = user32.NewProc("GetWindowDC")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	procPrintWindow         = user32.NewProc("PrintWindow")
)

const pwRenderFullContent = 0x00000002

type Window struct {
	Handle      win.HWND
	Title       string
	Bounds      image.Rectangle
	ProcessID   uint32
	ProcessName string
}

func Active() (*Window, error) {
	hwnd := win.GetForegroundWindow()
	if hwnd == 0 {
		return nil, fmt.Errorf("no active window")
	}
	return describeWindow(hwnd)
}

func List() ([]Window, error) {
	var items []Window
	callback := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		window, ok := inspectWindow(win.HWND(hwnd))
		if ok {
			items = append(items, window)
		}
		return 1
	})

	r1, _, err := procEnumWindows.Call(callback, 0)
	if r1 == 0 {
		if err != syscall.Errno(0) {
			return nil, fmt.Errorf("enum windows: %w", err)
		}
		return nil, fmt.Errorf("enum windows failed")
	}
	return items, nil
}

func FindFirst(titlePattern *regexp.Regexp, processNames ...string) (*Window, error) {
	items, err := List()
	if err != nil {
		return nil, err
	}

	normalizedProcesses := make(map[string]struct{}, len(processNames))
	for _, name := range processNames {
		name = normalizeProcessName(name)
		if name == "" {
			continue
		}
		normalizedProcesses[name] = struct{}{}
	}

	var best *Window
	bestScore := 0
	for i := range items {
		window := items[i]
		score := matchScore(window, titlePattern, normalizedProcesses)
		if score <= bestScore {
			continue
		}
		candidate := window
		best = &candidate
		bestScore = score
	}
	if best == nil {
		return nil, ErrTargetWindowNotFound
	}
	return best, nil
}

func Capture(window *Window) (image.Image, error) {
	if window == nil || window.Handle == 0 {
		return nil, fmt.Errorf("window handle is invalid")
	}
	width := window.Bounds.Dx()
	height := window.Bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("window bounds are invalid")
	}
	return captureWindow(window.Handle, width, height)
}

func TitleMatches(actual, target string) bool {
	actual = strings.TrimSpace(actual)
	target = strings.TrimSpace(target)
	return actual == target || strings.Contains(actual, target)
}

func TitleMatchesPattern(actual string, pattern *regexp.Regexp) bool {
	if pattern == nil {
		return false
	}
	return pattern.MatchString(strings.TrimSpace(actual))
}

func describeWindow(hwnd win.HWND) (*Window, error) {
	rect := win.RECT{}
	if !win.GetWindowRect(hwnd, &rect) {
		return nil, fmt.Errorf("get window rect failed")
	}

	var processID uint32
	win.GetWindowThreadProcessId(hwnd, &processID)

	return &Window{
		Handle:      hwnd,
		Title:       readWindowText(hwnd),
		Bounds:      image.Rect(int(rect.Left), int(rect.Top), int(rect.Right), int(rect.Bottom)),
		ProcessID:   processID,
		ProcessName: processName(processID),
	}, nil
}

func inspectWindow(hwnd win.HWND) (Window, bool) {
	if hwnd == 0 || !win.IsWindowVisible(hwnd) || win.IsIconic(hwnd) {
		return Window{}, false
	}
	if win.GetWindow(hwnd, win.GW_OWNER) != 0 {
		return Window{}, false
	}

	window, err := describeWindow(hwnd)
	if err != nil {
		return Window{}, false
	}
	if window.Bounds.Empty() || window.Bounds.Dx() <= 0 || window.Bounds.Dy() <= 0 {
		return Window{}, false
	}
	if strings.TrimSpace(window.Title) == "" && window.ProcessName == "" {
		return Window{}, false
	}
	return *window, true
}

func matchScore(window Window, titlePattern *regexp.Regexp, processNames map[string]struct{}) int {
	titleMatched := TitleMatchesPattern(window.Title, titlePattern)
	_, processMatched := processNames[normalizeProcessName(window.ProcessName)]
	if !titleMatched && !processMatched {
		return 0
	}

	score := 0
	if titleMatched {
		score += 100
	}
	if processMatched {
		score += 20
	}
	if titleMatched && strings.TrimSpace(window.Title) != "" {
		score += 5
	}
	if area := window.Bounds.Dx() * window.Bounds.Dy(); area > 0 {
		score += min(area/100000, 10)
	}
	return score
}

func processName(processID uint32) string {
	if processID == 0 {
		return ""
	}

	handle, err := xwindows.OpenProcess(xwindows.PROCESS_QUERY_LIMITED_INFORMATION, false, processID)
	if err != nil {
		return ""
	}
	defer xwindows.CloseHandle(handle)

	buf := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buf))
	if err := xwindows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return ""
	}
	return normalizeProcessName(filepath.Base(xwindows.UTF16ToString(buf[:size])))
}

func captureWindow(hwnd win.HWND, width, height int) (image.Image, error) {
	memDC := win.CreateCompatibleDC(0)
	if memDC == 0 {
		return nil, fmt.Errorf("create compatible dc failed")
	}
	defer win.DeleteDC(memDC)

	header := win.BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})),
		BiWidth:       int32(width),
		BiHeight:      -int32(height),
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: win.BI_RGB,
	}

	var bits unsafe.Pointer
	bitmap := win.CreateDIBSection(memDC, &header, win.DIB_RGB_COLORS, &bits, 0, 0)
	if bitmap == 0 {
		return nil, fmt.Errorf("create DIB section failed")
	}
	defer win.DeleteObject(win.HGDIOBJ(bitmap))

	previous := win.SelectObject(memDC, win.HGDIOBJ(bitmap))
	if previous == 0 {
		return nil, fmt.Errorf("select bitmap into dc failed")
	}
	defer win.SelectObject(memDC, previous)

	printed := printWindow(hwnd, memDC)
	img := dibToImage(bits, width, height)
	if !printed || isImageBlank(img) {
		if err := bitbltWindow(hwnd, memDC, width, height); err != nil {
			if printed {
				return img, nil
			}
			return nil, err
		}
		img = dibToImage(bits, width, height)
	}

	if bits == nil {
		return nil, fmt.Errorf("capture returned no pixel buffer")
	}

	return img, nil
}

func printWindow(hwnd win.HWND, dc win.HDC) bool {
	for _, flags := range []uintptr{pwRenderFullContent, 0} {
		r1, _, _ := procPrintWindow.Call(uintptr(hwnd), uintptr(dc), flags)
		if r1 != 0 {
			return true
		}
	}
	return false
}

func bitbltWindow(hwnd win.HWND, dc win.HDC, width, height int) error {
	src, _, err := procGetWindowDC.Call(uintptr(hwnd))
	if src == 0 {
		return fmt.Errorf("get window dc failed: %w", err)
	}
	defer win.ReleaseDC(hwnd, win.HDC(src))

	if ok := win.BitBlt(dc, 0, 0, int32(width), int32(height), win.HDC(src), 0, 0, win.SRCCOPY); !ok {
		return fmt.Errorf("bitblt window failed")
	}
	return nil
}

func dibToImage(bits unsafe.Pointer, width, height int) *image.NRGBA {
	raw := unsafe.Slice((*byte)(bits), width*height*4)
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcRow := raw[y*width*4 : (y+1)*width*4]
		dstRow := img.Pix[y*img.Stride : y*img.Stride+width*4]
		for x := 0; x < width*4; x += 4 {
			dstRow[x+0] = srcRow[x+2]
			dstRow[x+1] = srcRow[x+1]
			dstRow[x+2] = srcRow[x+0]
			alpha := srcRow[x+3]
			if alpha == 0 {
				alpha = 255
			}
			dstRow[x+3] = alpha
		}
	}
	return img
}

func isImageBlank(img *image.NRGBA) bool {
	bounds := img.Bounds()
	strideX := max(bounds.Dx()/12, 1)
	strideY := max(bounds.Dy()/12, 1)
	first := img.NRGBAAt(0, 0)
	for y := 0; y < bounds.Dy(); y += strideY {
		for x := 0; x < bounds.Dx(); x += strideX {
			if img.NRGBAAt(x, y) != first {
				return false
			}
		}
	}
	return true
}

func normalizeProcessName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func readWindowText(hwnd win.HWND) string {
	length, _, _ := procGetWindowTextLength.Call(uintptr(hwnd))
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, length+1)
	procGetWindowTextW.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	return xwindows.UTF16ToString(buffer)
}
