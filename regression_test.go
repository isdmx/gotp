package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"golang.org/x/image/draw"
)

// renderNative renders a BitMatrix at one pixel per module.
func renderNative(bm *gozxing.BitMatrix) *image.Gray {
	w, h := bm.GetWidth(), bm.GetHeight()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.White
			if bm.Get(x, y) {
				c = color.Black
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func TestDecodeQRJPEGHighVersion(t *testing.T) {
	// A 20-account migration payload produces a high-version QR code that
	// gozxing cannot decode at its native size on a lossy JPEG. The scale
	// pyramid must recover it.
	var p []byte
	for i := 0; i < 20; i++ {
		op := appendBytesField(nil, 1, []byte(fmt.Sprintf("JBSWY3DPEHPK3PXP%d", i)))
		op = appendBytesField(op, 2, []byte(fmt.Sprintf("user%d@gmail.com", i)))
		op = appendBytesField(op, 3, []byte("Google"))
		op = appendVarintField(op, 4, uint64(AlgoSHA1))
		op = appendVarintField(op, 5, 6)
		op = appendVarintField(op, 6, uint64(OtpTOTP))
		p = appendBytesField(p, 1, op)
	}
	p = appendVarintField(p, 2, 1)

	migrationURL := "otpauth-migration://offline?data=" + base64.StdEncoding.EncodeToString(p)
	bm, err := qrcode.NewQRCodeWriter().EncodeWithoutHint(migrationURL, gozxing.BarcodeFormat_QR_CODE, 0, 0)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Smooth-scale to 726x726, then lossy JPEG.
	src := renderNative(bm)
	scaled := image.NewRGBA(image.Rect(0, 0, 726, 726))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, scaled, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("jpeg encode: %v", err)
	}
	img, err := jpeg.Decode(&buf)
	if err != nil {
		t.Fatalf("jpeg decode: %v", err)
	}

	got, err := decodeQR(img)
	if err != nil {
		t.Fatalf("decodeQR: %v", err)
	}
	if got != migrationURL {
		t.Errorf("decodeQR = %q, want %q", got, migrationURL)
	}
}
