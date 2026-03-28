package service

import (
	"errors"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectLoginModalBounds(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1600, 900))
	fillRect(img, img.Bounds(), color.NRGBA{R: 18, G: 34, B: 56, A: 255})

	modal := image.Rect(470, 180, 1130, 700)
	drawModalFrame(img, modal)

	got, ok := detectLoginModalBounds(img)
	if !ok {
		t.Fatalf("expected modal detection to succeed")
	}
	if absInt(got.Min.X-modal.Min.X) > 6 || absInt(got.Min.Y-modal.Min.Y) > 6 || absInt(got.Max.X-modal.Max.X) > 6 || absInt(got.Max.Y-modal.Max.Y) > 6 {
		t.Fatalf("unexpected modal bounds: got=%v want~=%v", got, modal)
	}
}

func TestDialogShowsExpandedQRCode(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1200, 800))
	fillRect(img, img.Bounds(), color.NRGBA{R: 20, G: 60, B: 96, A: 255})

	modal := image.Rect(260, 120, 940, 680)
	drawModalFrame(img, modal)
	qrArea := relativeRect(modal, 0.30, 0.18, 0.70, 0.60)
	fillRect(img, qrArea, color.NRGBA{R: 245, G: 245, B: 245, A: 255})
	for y := qrArea.Min.Y; y < qrArea.Max.Y; y += 20 {
		for x := qrArea.Min.X; x < qrArea.Max.X; x += 20 {
			fillRect(img, image.Rect(x, y, minInt(x+10, qrArea.Max.X), minInt(y+10, qrArea.Max.Y)), color.NRGBA{A: 255})
		}
	}

	if !dialogShowsExpandedQRCode(img, modal) {
		t.Fatalf("expected expanded QR detection to succeed")
	}
}

func TestDialogShowsExpiredQRCode(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1200, 800))
	fillRect(img, img.Bounds(), color.NRGBA{R: 20, G: 60, B: 96, A: 255})

	modal := image.Rect(260, 120, 940, 680)
	drawModalFrame(img, modal)
	qrArea := relativeRect(modal, 0.30, 0.18, 0.70, 0.60)
	fillRect(img, qrArea, color.NRGBA{R: 245, G: 245, B: 245, A: 255})
	refreshArea := relativeRect(modal, 0.425, 0.33, 0.575, 0.49)
	fillRect(img, refreshArea, color.NRGBA{R: 34, G: 190, B: 235, A: 255})

	if !dialogShowsExpandedQRCode(img, modal) {
		t.Fatalf("expected expanded QR detection to succeed")
	}
	if !dialogShowsExpiredQRCode(img, modal) {
		t.Fatalf("expected expired QR detection to succeed")
	}
}

func TestDetectRefreshButtonPoint(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1200, 800))
	fillRect(img, img.Bounds(), color.NRGBA{R: 20, G: 60, B: 96, A: 255})

	refreshArea := image.Rect(540, 300, 660, 420)
	fillRect(img, refreshArea, color.NRGBA{R: 34, G: 190, B: 235, A: 255})

	point, ok := detectRefreshButtonPoint(img)
	if !ok {
		t.Fatalf("expected refresh button detection to succeed")
	}
	if !point.In(refreshArea) {
		t.Fatalf("detected refresh point %v should land inside %v", point, refreshArea)
	}
}

func TestShouldAttemptWindowQRExpand(t *testing.T) {
	if shouldAttemptWindowQRExpand(nil) {
		t.Fatalf("nil error should not trigger expand attempt")
	}
	if shouldAttemptWindowQRExpand(errors.New("decode qr code: ticket not found in decoded qr contents")) {
		t.Fatalf("non-empty qr decode should not trigger expand attempt")
	}
	if !shouldAttemptWindowQRExpand(errors.New("decode qr code: no qr code found")) {
		t.Fatalf("missing qr should trigger expand attempt")
	}
}

func TestClassifyWindowQRStateExpandedUnreadable(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1200, 800))
	fillRect(img, img.Bounds(), color.NRGBA{R: 20, G: 60, B: 96, A: 255})

	modal := image.Rect(260, 120, 940, 680)
	drawModalFrame(img, modal)
	qrArea := relativeRect(modal, 0.30, 0.18, 0.70, 0.60)
	fillRect(img, qrArea, color.NRGBA{R: 245, G: 245, B: 245, A: 255})
	for y := qrArea.Min.Y; y < qrArea.Max.Y; y += 20 {
		for x := qrArea.Min.X; x < qrArea.Max.X; x += 20 {
			fillRect(img, image.Rect(x, y, minInt(x+8, qrArea.Max.X), minInt(y+8, qrArea.Max.Y)), color.NRGBA{A: 255})
		}
	}

	ref, ok := classifyWindowQRState(img)
	if !ok {
		t.Fatalf("expected qr state classification to succeed")
	}
	if ref.Code != "backend.hint.qr_visible_but_unreadable" {
		t.Fatalf("unexpected code: %s", ref.Code)
	}
}

func TestClassifyWindowQRStateNeedsManualExpand(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1200, 800))
	fillRect(img, img.Bounds(), color.NRGBA{R: 20, G: 60, B: 96, A: 255})

	modal := image.Rect(260, 120, 940, 680)
	drawModalFrame(img, modal)
	fillRect(img, image.Rect(510, 250, 690, 320), color.NRGBA{R: 34, G: 190, B: 235, A: 255})
	fillRect(img, image.Rect(380, 360, 820, 430), color.NRGBA{R: 28, G: 64, B: 108, A: 255})
	fillRect(img, image.Rect(380, 460, 820, 530), color.NRGBA{R: 28, G: 64, B: 108, A: 255})
	fillRect(img, relativeRect(modal, 0.08, 0.67, 0.92, 0.84), color.NRGBA{R: 246, G: 206, B: 74, A: 255})

	ref, ok := classifyWindowQRState(img)
	if !ok {
		t.Fatalf("expected qr state classification to succeed")
	}
	if ref.Code != "backend.hint.qr_expand_manual" {
		t.Fatalf("unexpected code: %s", ref.Code)
	}
}

func TestDialogShowsLoginActionBar(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1200, 800))
	fillRect(img, img.Bounds(), color.NRGBA{R: 20, G: 60, B: 96, A: 255})

	modal := image.Rect(260, 120, 940, 680)
	drawModalFrame(img, modal)
	fillRect(img, relativeRect(modal, 0.08, 0.67, 0.92, 0.84), color.NRGBA{R: 246, G: 206, B: 74, A: 255})

	if !dialogShowsLoginActionBar(img, modal) {
		t.Fatalf("expected yellow action bar detection to succeed")
	}
}

func TestClassifyWindowQRStateFixtures(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "tmp", "qr-fixtures")
	cases := []struct {
		name     string
		filename string
		wantCode string
	}{
		{name: "login form", filename: "login-form.png", wantCode: "backend.hint.qr_expand_manual"},
		{name: "expanded qr", filename: "qr-expanded.png", wantCode: "backend.hint.qr_visible_but_unreadable"},
		{name: "expired qr", filename: "qr-expired.png", wantCode: "backend.hint.qr_refresh_manual"},
	}

	for _, tc := range cases {
		path := filepath.Join(fixtureDir, tc.filename)
		file, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				t.Skipf("fixture missing: %s", path)
			}
			t.Fatalf("open fixture %s: %v", path, err)
		}
		img, _, err := image.Decode(file)
		_ = file.Close()
		if err != nil {
			t.Fatalf("decode fixture %s: %v", path, err)
		}

		ref, ok := classifyWindowQRState(img)
		if !ok {
			t.Fatalf("%s: expected qr state classification to succeed", tc.name)
		}
		if ref.Code != tc.wantCode {
			modal, modalOK := detectLoginModalBounds(img)
			if !modalOK {
				modal = estimateCenteredLoginModalBounds(img)
			}
			t.Fatalf(
				"%s: unexpected code: got=%s want=%s modal=%v modalOK=%v loginAction=%v expanded=%v expired=%v",
				tc.name,
				ref.Code,
				tc.wantCode,
				modal,
				modalOK,
				dialogShowsLoginActionBar(img, modal),
				dialogShowsExpandedQRCode(img, modal),
				dialogShowsExpiredQRCode(img, modal),
			)
		}
	}
}

func drawModalFrame(img *image.NRGBA, modal image.Rectangle) {
	frame := color.NRGBA{R: 20, G: 196, B: 245, A: 255}
	fillRect(img, modal, color.NRGBA{R: 16, G: 70, B: 112, A: 255})
	fillRect(img, image.Rect(modal.Min.X, modal.Min.Y, modal.Max.X, modal.Min.Y+4), frame)
	fillRect(img, image.Rect(modal.Min.X, modal.Max.Y-4, modal.Max.X, modal.Max.Y), frame)
	fillRect(img, image.Rect(modal.Min.X, modal.Min.Y, modal.Min.X+4, modal.Max.Y), frame)
	fillRect(img, image.Rect(modal.Max.X-4, modal.Min.Y, modal.Max.X, modal.Max.Y), frame)
}

func fillRect(img *image.NRGBA, rect image.Rectangle, c color.NRGBA) {
	rect = rect.Intersect(img.Bounds())
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}
