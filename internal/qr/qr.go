package qr

import (
	"bytes"
	"fmt"
	"image"
	stddraw "image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"strings"

	"github.com/makiuchi-d/gozxing"
	qrmulti "github.com/makiuchi-d/gozxing/multi/qrcode"
	qrcodereader "github.com/makiuchi-d/gozxing/qrcode"
	xdraw "golang.org/x/image/draw"
)

func DecodeImage(img image.Image) (string, error) {
	texts, err := decodeTexts(img)
	if err != nil {
		return "", err
	}
	return texts[0], nil
}

func DecodeBytes(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode image bytes: %w", err)
	}
	return DecodeImage(img)
}

func ExtractTicket(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse qr url: %w", err)
	}
	ticket := parsed.Query().Get("ticket")
	if ticket == "" {
		return "", fmt.Errorf("ticket not found in qr url")
	}
	return ticket, nil
}

func DecodeTicketFromImage(img image.Image) (ticket string, rawURL string, err error) {
	texts, err := decodeTexts(img)
	if err != nil {
		return "", "", err
	}

	for _, text := range texts {
		ticket, err = ExtractTicket(text)
		if err == nil {
			return ticket, text, nil
		}
	}
	return "", texts[0], fmt.Errorf("ticket not found in decoded qr contents")
}

func DecodeTicketFromBytes(data []byte) (ticket string, rawURL string, err error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", "", fmt.Errorf("decode image bytes: %w", err)
	}
	return DecodeTicketFromImage(img)
}

func decodeTexts(img image.Image) ([]string, error) {
	var (
		texts    []string
		seen     = map[string]struct{}{}
		firstErr error
	)

	for _, candidate := range candidateImages(img) {
		found, err := decodeCandidate(candidate)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		for _, text := range found {
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			if _, ok := seen[text]; ok {
				continue
			}
			seen[text] = struct{}{}
			texts = append(texts, text)
		}
	}
	if len(texts) > 0 {
		return texts, nil
	}
	if firstErr == nil {
		firstErr = fmt.Errorf("decode qr code: no qr code found")
	}
	return nil, firstErr
}

func decodeCandidate(img image.Image) ([]string, error) {
	bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil, fmt.Errorf("build binary bitmap: %w", err)
	}

	var (
		results  []string
		seen     = map[string]struct{}{}
		lastErr  error
		reader   = qrcodereader.NewQRCodeReader()
		multi    = qrmulti.NewQRCodeMultiReader()
		hintSets = []map[gozxing.DecodeHintType]interface{}{
			nil,
			{gozxing.DecodeHintType_TRY_HARDER: true},
			{gozxing.DecodeHintType_PURE_BARCODE: true},
			{
				gozxing.DecodeHintType_TRY_HARDER:   true,
				gozxing.DecodeHintType_PURE_BARCODE: true,
			},
		}
	)

	addText := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		if _, ok := seen[text]; ok {
			return
		}
		seen[text] = struct{}{}
		results = append(results, text)
	}

	for _, hints := range hintSets {
		result, err := reader.Decode(bitmap, hints)
		if err != nil {
			lastErr = err
		} else if result != nil {
			addText(result.GetText())
		}

		multiResults, err := multi.DecodeMultiple(bitmap, hints)
		if err != nil {
			if lastErr == nil {
				lastErr = err
			}
		} else {
			for _, item := range multiResults {
				if item != nil {
					addText(item.GetText())
				}
			}
		}
	}

	if len(results) > 0 {
		return results, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("decode qr code: no qr code found")
	}
	return nil, fmt.Errorf("decode qr code: %w", lastErr)
}

func candidateImages(img image.Image) []image.Image {
	base := cloneImage(img)
	if base == nil {
		return nil
	}

	rects := candidateRects(base.Bounds())
	out := make([]image.Image, 0, len(rects)*3)
	for _, rect := range rects {
		candidate := cropImage(base, rect)
		if candidate == nil {
			continue
		}
		out = append(out, candidate)

		size := candidate.Bounds().Size()
		minSide := min(size.X, size.Y)
		if minSide < 900 {
			out = append(out, scaleImage(candidate, 2))
		}
		if minSide < 500 {
			out = append(out, scaleImage(candidate, 3))
		}
	}
	return out
}

func candidateRects(bounds image.Rectangle) []image.Rectangle {
	var rects []image.Rectangle
	seen := map[string]struct{}{}
	add := func(rect image.Rectangle) {
		rect = rect.Intersect(bounds)
		if rect.Empty() || rect.Dx() <= 0 || rect.Dy() <= 0 {
			return
		}
		key := fmt.Sprintf("%d,%d,%d,%d", rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		rects = append(rects, rect)
	}

	add(bounds)
	add(centerRect(bounds, 0.7))
	add(centerRect(bounds, 0.5))

	width := bounds.Dx()
	height := bounds.Dy()
	cropWidth := max(int(float64(width)*0.6), min(width, 320))
	cropHeight := max(int(float64(height)*0.6), min(height, 320))
	xPositions := []int{bounds.Min.X, bounds.Min.X + (width-cropWidth)/2, bounds.Max.X - cropWidth}
	yPositions := []int{bounds.Min.Y, bounds.Min.Y + (height-cropHeight)/2, bounds.Max.Y - cropHeight}
	for _, x := range xPositions {
		for _, y := range yPositions {
			add(image.Rect(x, y, x+cropWidth, y+cropHeight))
		}
	}
	return rects
}

func centerRect(bounds image.Rectangle, ratio float64) image.Rectangle {
	width := max(int(float64(bounds.Dx())*ratio), 1)
	height := max(int(float64(bounds.Dy())*ratio), 1)
	left := bounds.Min.X + (bounds.Dx()-width)/2
	top := bounds.Min.Y + (bounds.Dy()-height)/2
	return image.Rect(left, top, left+width, top+height)
}

func cloneImage(img image.Image) *image.NRGBA {
	if img == nil {
		return nil
	}
	bounds := img.Bounds()
	clone := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	stddraw.Draw(clone, clone.Bounds(), img, bounds.Min, stddraw.Src)
	return clone
}

func cropImage(img image.Image, rect image.Rectangle) image.Image {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() || rect.Dx() <= 0 || rect.Dy() <= 0 {
		return nil
	}
	cropped := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	stddraw.Draw(cropped, cropped.Bounds(), img, rect.Min, stddraw.Src)
	return cropped
}

func scaleImage(img image.Image, factor int) image.Image {
	if factor <= 1 {
		return img
	}
	bounds := img.Bounds()
	scaled := image.NewNRGBA(image.Rect(0, 0, bounds.Dx()*factor, bounds.Dy()*factor))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), img, bounds, xdraw.Src, nil)
	return scaled
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
