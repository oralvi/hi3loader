package service

import (
	"image"
	"image/color"
	"strings"

	"hi3loader/internal/winwindow"
)

func (s *Service) tryExpandWindowQRCode(img image.Image) (MessageRef, error) {
	ref, _ := classifyWindowQRState(img)
	if ref.Code == "" {
		return MessageRef{}, nil
	}
	switch ref.Code {
	case "backend.hint.qr_panel_unrecognized":
		s.logf("game window captured, but login panel heuristics did not match")
		s.noteWindowQRHint(
			ref,
			"Login panel was not recognized in the captured game window. Open the login window and try again.",
		)
	case "backend.hint.qr_visible_but_unreadable":
		s.logf("login panel detected with QR area visible, but no usable QR was decoded")
		s.noteWindowQRHint(
			ref,
			"A QR area is visible, but no usable QR code was decoded. Make sure the QR is clear and retry.",
		)
	case "backend.hint.qr_expand_manual":
		s.logf("login panel detected without QR login; manual QR switch is required")
		s.noteWindowQRHint(
			ref,
			"QR login is not open in the game window. Please switch to QR login manually.",
		)
	}
	return ref, nil
}

func (s *Service) tryRefreshExpiredWindowQRCode(window *winwindow.Window, img image.Image) (MessageRef, error) {
	if img == nil {
		captured, err := winwindow.Capture(window)
		if err != nil {
			return MessageRef{}, err
		}
		img = captured
	}

	ref := newMessageRef("backend.hint.qr_refresh_manual", nil)
	modal, ok := detectLoginModalBounds(img)
	if !ok {
		s.noteWindowQRHint(
			ref,
			"The QR code has expired. Please click Refresh in the game window.",
		)
		return ref, nil
	}
	if !dialogShowsExpandedQRCode(img, modal) {
		return MessageRef{}, nil
	}
	if refreshPoint, ok := detectRefreshButtonPoint(relativeImageView{src: img, rect: modal}); ok && refreshPoint.In(modal) {
		s.logf("expired QR visual detected at (%d,%d)", refreshPoint.X, refreshPoint.Y)
		s.logf("expired QR detected in the game window; manual refresh is required")
		s.noteWindowQRHint(
			ref,
			"The QR code has expired. Please click Refresh in the game window.",
		)
		return ref, nil
	}
	if !dialogShowsExpiredQRCode(img, modal) {
		return MessageRef{}, nil
	}

	s.logf("expired QR detected in the game window; manual refresh is required")
	s.noteWindowQRHint(
		ref,
		"The QR code has expired. Please click Refresh in the game window.",
	)
	return ref, nil
}

func classifyWindowQRState(img image.Image) (MessageRef, bool) {
	if img == nil {
		return MessageRef{}, false
	}
	modal, ok := detectLoginModalBounds(img)
	if !ok {
		modal, ok = detectEdgeFramedModalBounds(img)
		if !ok {
			modal = estimateCenteredLoginModalBounds(img)
		}
		if modal.Empty() {
			return newMessageRef("backend.hint.qr_panel_unrecognized", nil), true
		}
		if dialogShowsLoginActionBar(img, modal) {
			return newMessageRef("backend.hint.qr_expand_manual", nil), true
		}
		if dialogShowsExpiredQRCode(img, modal) {
			return newMessageRef("backend.hint.qr_refresh_manual", nil), true
		}
		if dialogShowsExpandedQRCode(img, modal) {
			if point, ok := detectRefreshButtonPoint(relativeImageView{src: img, rect: modal}); ok {
				if point.In(modal) {
					return newMessageRef("backend.hint.qr_refresh_manual", nil), true
				}
			}
			return newMessageRef("backend.hint.qr_visible_but_unreadable", nil), true
		}
		return newMessageRef("backend.hint.qr_panel_unrecognized", nil), true
	}
	if dialogShowsLoginActionBar(img, modal) {
		return newMessageRef("backend.hint.qr_expand_manual", nil), true
	}
	if dialogShowsExpiredQRCode(img, modal) {
		return newMessageRef("backend.hint.qr_refresh_manual", nil), true
	}
	if dialogShowsExpandedQRCode(img, modal) {
		if point, ok := detectRefreshButtonPoint(relativeImageView{src: img, rect: modal}); ok {
			if point.In(modal) {
				return newMessageRef("backend.hint.qr_refresh_manual", nil), true
			}
		}
		return newMessageRef("backend.hint.qr_visible_but_unreadable", nil), true
	}
	return newMessageRef("backend.hint.qr_expand_manual", nil), true
}

func (s *Service) noteWindowQRHint(ref MessageRef, logText string) {
	s.setHint(ref, logText)
}

func shouldAttemptWindowQRExpand(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "no qr code found")
}

func detectLoginModalBounds(img image.Image) (image.Rectangle, bool) {
	if img == nil {
		return image.Rectangle{}, false
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width < 480 || height < 320 {
		return image.Rectangle{}, false
	}

	startX := bounds.Min.X + width/10
	endX := bounds.Min.X + (width*9)/10
	startY := bounds.Min.Y + height/14
	endY := bounds.Min.Y + height/2
	minRun := maxInt(width/4, 240)

	topY := 0
	bestLeft := 0
	bestRight := 0
	bestLen := 0
	for y := startY; y < endY; y++ {
		left, right, run := longestCyanAccentRun(img, y, startX, endX)
		if run > bestLen {
			bestLen = run
			topY = y
			bestLeft = left
			bestRight = right
		}
	}
	if bestLen < minRun {
		return image.Rectangle{}, false
	}

	searchBottomStart := topY + maxInt(height/5, 120)
	searchBottomEnd := minInt(bounds.Min.Y+(height*9)/10, topY+(height*3)/4)
	expectedRun := maxInt((bestLen*4)/5, minRun)
	bottomY := 0
	for y := searchBottomStart; y < searchBottomEnd; y++ {
		left, right, run := longestCyanAccentRun(img, y, startX, endX)
		if run < expectedRun {
			continue
		}
		if absInt(left-bestLeft) > maxInt(width/20, 48) || absInt(right-bestRight) > maxInt(width/20, 48) {
			continue
		}
		bottomY = y
	}
	if bottomY == 0 {
		return image.Rectangle{}, false
	}

	rect := image.Rect(bestLeft, topY, bestRight, bottomY)
	if rect.Dx() < maxInt(width/4, 280) || rect.Dx() > (width*4)/5 {
		return image.Rectangle{}, false
	}
	if rect.Dy() < maxInt(height/4, 220) || rect.Dy() > (height*4)/5 {
		return image.Rectangle{}, false
	}

	centerX := rect.Min.X + rect.Dx()/2
	if centerX < bounds.Min.X+width/4 || centerX > bounds.Min.X+(width*3)/4 {
		return image.Rectangle{}, false
	}
	return rect, true
}

func dialogShowsExpandedQRCode(img image.Image, modal image.Rectangle) bool {
	if img == nil || modal.Empty() {
		return false
	}
	sample := relativeRect(modal, 0.30, 0.18, 0.70, 0.60)
	if sample.Empty() || sample.Dx() <= 0 || sample.Dy() <= 0 {
		return false
	}

	total := 0
	bright := 0
	dark := 0
	refreshBlue := 0
	for y := sample.Min.Y; y < sample.Max.Y; y += 2 {
		for x := sample.Min.X; x < sample.Max.X; x += 2 {
			r, g, b, _ := img.At(x, y).RGBA()
			rr := int(r >> 8)
			gg := int(g >> 8)
			bb := int(b >> 8)
			total++
			if rr >= 215 && gg >= 215 && bb >= 215 {
				bright++
			}
			if rr <= 55 && gg <= 55 && bb <= 55 {
				dark++
			}
			if isRefreshBlue(img.At(x, y)) {
				refreshBlue++
			}
		}
	}
	if total == 0 {
		return false
	}
	brightRatio := float64(bright) / float64(total)
	darkRatio := float64(dark) / float64(total)
	refreshBlueRatio := float64(refreshBlue) / float64(total)
	return brightRatio >= 0.14 && (darkRatio >= 0.03 || refreshBlueRatio >= 0.04)
}

func dialogShowsLoginActionBar(img image.Image, modal image.Rectangle) bool {
	return detectYellowActionBar(img, relativeRect(modal, 0.06, 0.58, 0.94, 0.84), modal.Dx(), modal.Dy())
}

func estimateCenteredLoginModalBounds(img image.Image) image.Rectangle {
	if img == nil {
		return image.Rectangle{}
	}
	bounds := img.Bounds()
	if bounds.Empty() {
		return image.Rectangle{}
	}
	return relativeRect(bounds, 0.22, 0.14, 0.78, 0.86)
}

func detectEdgeFramedModalBounds(img image.Image) (image.Rectangle, bool) {
	if img == nil {
		return image.Rectangle{}, false
	}
	bounds := img.Bounds()
	if bounds.Empty() || bounds.Dx() < 320 || bounds.Dy() < 280 {
		return image.Rectangle{}, false
	}
	marginX := maxInt(bounds.Dx()/40, 6)
	marginY := maxInt(bounds.Dy()/40, 6)

	top := image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Min.Y+marginY)
	bottom := image.Rect(bounds.Min.X, bounds.Max.Y-marginY, bounds.Max.X, bounds.Max.Y)
	left := image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+marginX, bounds.Max.Y)
	right := image.Rect(bounds.Max.X-marginX, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)

	if cyanCoverage(img, top) < 0.08 || cyanCoverage(img, bottom) < 0.08 {
		return image.Rectangle{}, false
	}
	if cyanCoverage(img, left) < 0.04 || cyanCoverage(img, right) < 0.04 {
		return image.Rectangle{}, false
	}
	return bounds, true
}

func cyanCoverage(img image.Image, rect image.Rectangle) float64 {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return 0
	}
	total := 0
	hit := 0
	for y := rect.Min.Y; y < rect.Max.Y; y += 2 {
		for x := rect.Min.X; x < rect.Max.X; x += 2 {
			total++
			if isCyanAccent(img.At(x, y)) {
				hit++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hit) / float64(total)
}

func detectYellowActionBar(img image.Image, search image.Rectangle, refWidth, refHeight int) bool {
	if img == nil || search.Empty() {
		return false
	}
	if search.Empty() || search.Dx() <= 0 || search.Dy() <= 0 {
		return false
	}

	minX := search.Max.X
	minY := search.Max.Y
	maxX := search.Min.X
	maxY := search.Min.Y
	count := 0
	total := 0
	for y := search.Min.Y; y < search.Max.Y; y += 2 {
		for x := search.Min.X; x < search.Max.X; x += 2 {
			total++
			if !isActionYellow(img.At(x, y)) {
				continue
			}
			count++
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if total == 0 || count < 120 || minX >= maxX || minY >= maxY {
		return false
	}

	boxWidth := maxX - minX
	boxHeight := maxY - minY
	coverage := float64(count) / float64(total)
	return boxWidth >= int(float64(refWidth)*0.22) &&
		boxHeight >= int(float64(refHeight)*0.05) &&
		coverage >= 0.08
}

func dialogShowsExpiredQRCode(img image.Image, modal image.Rectangle) bool {
	if img == nil || modal.Empty() {
		return false
	}
	sample := relativeRect(modal, 0.425, 0.33, 0.575, 0.49)
	if sample.Empty() || sample.Dx() <= 0 || sample.Dy() <= 0 {
		return false
	}

	total := 0
	refreshBlue := 0
	for y := sample.Min.Y; y < sample.Max.Y; y += 2 {
		for x := sample.Min.X; x < sample.Max.X; x += 2 {
			total++
			if isRefreshBlue(img.At(x, y)) {
				refreshBlue++
			}
		}
	}
	if total == 0 {
		return false
	}
	return float64(refreshBlue)/float64(total) >= 0.22
}

func detectRefreshButtonPoint(img image.Image) (image.Point, bool) {
	if img == nil {
		return image.Point{}, false
	}
	search := relativeRect(img.Bounds(), 0.30, 0.18, 0.70, 0.68)
	if search.Empty() {
		return image.Point{}, false
	}

	minX := search.Max.X
	minY := search.Max.Y
	maxX := search.Min.X
	maxY := search.Min.Y
	count := 0
	for y := search.Min.Y; y < search.Max.Y; y += 2 {
		for x := search.Min.X; x < search.Max.X; x += 2 {
			if !isRefreshBlue(img.At(x, y)) {
				continue
			}
			count++
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if count < 40 || minX >= maxX || minY >= maxY {
		return image.Point{}, false
	}
	if maxX-minX < 18 || maxY-minY < 18 {
		return image.Point{}, false
	}
	return image.Pt((minX+maxX)/2, (minY+maxY)/2), true
}

type relativeImageView struct {
	src  image.Image
	rect image.Rectangle
}

func (v relativeImageView) ColorModel() color.Model {
	return v.src.ColorModel()
}

func (v relativeImageView) Bounds() image.Rectangle {
	return v.rect
}

func (v relativeImageView) At(x, y int) color.Color {
	return v.src.At(x, y)
}

func relativeRect(bounds image.Rectangle, left, top, right, bottom float64) image.Rectangle {
	width := bounds.Dx()
	height := bounds.Dy()
	return image.Rect(
		bounds.Min.X+int(float64(width)*left),
		bounds.Min.Y+int(float64(height)*top),
		bounds.Min.X+int(float64(width)*right),
		bounds.Min.Y+int(float64(height)*bottom),
	)
}

func longestCyanAccentRun(img image.Image, y, startX, endX int) (int, int, int) {
	bestStart := startX
	bestEnd := startX
	bestLen := 0
	currentStart := -1

	for x := startX; x < endX; x++ {
		if isCyanAccent(img.At(x, y)) {
			if currentStart < 0 {
				currentStart = x
			}
			continue
		}
		if currentStart >= 0 {
			runLen := x - currentStart
			if runLen > bestLen {
				bestLen = runLen
				bestStart = currentStart
				bestEnd = x
			}
			currentStart = -1
		}
	}
	if currentStart >= 0 {
		runLen := endX - currentStart
		if runLen > bestLen {
			bestLen = runLen
			bestStart = currentStart
			bestEnd = endX
		}
	}
	return bestStart, bestEnd, bestLen
}

func isCyanAccent(c colorLike) bool {
	r, g, b, _ := c.RGBA()
	rr := int(r >> 8)
	gg := int(g >> 8)
	bb := int(b >> 8)
	maxChannel := maxInt(rr, maxInt(gg, bb))
	minChannel := minInt(rr, minInt(gg, bb))
	return bb >= 170 && gg >= 120 && rr <= 110 && maxChannel-minChannel >= 70
}

func isRefreshBlue(c colorLike) bool {
	r, g, b, _ := c.RGBA()
	rr := int(r >> 8)
	gg := int(g >> 8)
	bb := int(b >> 8)
	return rr <= 90 && gg >= 145 && bb >= 190
}

func isActionYellow(c colorLike) bool {
	r, g, b, _ := c.RGBA()
	rr := int(r >> 8)
	gg := int(g >> 8)
	bb := int(b >> 8)
	return rr >= 185 && gg >= 145 && bb <= 110
}

type colorLike interface {
	RGBA() (r, g, b, a uint32)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
