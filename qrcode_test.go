package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// bitMatrixToImage renders a gozxing BitMatrix as a grayscale image, scaling
// each QR module to scale pixels.
func bitMatrixToImage(bm *gozxing.BitMatrix, scale int) image.Image {
	w, h := bm.GetWidth()*scale, bm.GetHeight()*scale
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < bm.GetHeight(); y++ {
		for x := 0; x < bm.GetWidth(); x++ {
			c := color.White
			if bm.Get(x, y) {
				c = color.Black
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					img.Set(x*scale+dx, y*scale+dy, c)
				}
			}
		}
	}
	return img
}

func TestDecodeQR(t *testing.T) {
	contents := "otpauth-migration://offline?data=SGVsbG8h"
	bm, err := qrcode.NewQRCodeWriter().EncodeWithoutHint(contents, gozxing.BarcodeFormat_QR_CODE, 256, 256)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	got, err := decodeQR(bitMatrixToImage(bm, 8))
	if err != nil {
		t.Fatalf("decodeQR: %v", err)
	}
	if got != contents {
		t.Errorf("decodeQR = %q, want %q", got, contents)
	}
}

func TestDecodeQRHighVersion(t *testing.T) {
	// A migration payload with many accounts produces a high-version QR code.
	// gozxing's default detector fails on some high versions due to a dimension
	// rounding bug, so decodeQR must fall back to PURE_BARCODE decoding.
	var p []byte
	for i := 0; i < 24; i++ {
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

	got, err := decodeQR(bitMatrixToImage(bm, 4))
	if err != nil {
		t.Fatalf("decodeQR: %v", err)
	}
	if got != migrationURL {
		t.Errorf("decodeQR = %q, want %q", got, migrationURL)
	}
}

func TestExtractMigrationData(t *testing.T) {
	got, err := extractMigrationData("otpauth-migration://offline?data=SGVsbG8h")
	if err != nil {
		t.Fatalf("extractMigrationData: %v", err)
	}
	if string(got) != "Hello!" {
		t.Errorf("extractMigrationData = %q, want %q", got, "Hello!")
	}
}

func TestExtractMigrationDataUnpadded(t *testing.T) {
	got, err := extractMigrationData("otpauth-migration://offline?data=SGVsbG8")
	if err != nil {
		t.Fatalf("extractMigrationData: %v", err)
	}
	if string(got) != "Hello" {
		t.Errorf("extractMigrationData = %q, want %q", got, "Hello")
	}
}

func TestExtractMigrationDataURLEncoded(t *testing.T) {
	// Base64 of {0xFB, 0xFF, 0xFE} is "+//+", percent-encoded in the URL.
	got, err := extractMigrationData("otpauth-migration://offline?data=%2B%2F%2F%2B")
	if err != nil {
		t.Fatalf("extractMigrationData: %v", err)
	}
	want := []byte{0xFB, 0xFF, 0xFE}
	if !bytes.Equal(got, want) {
		t.Errorf("extractMigrationData = %v, want %v", got, want)
	}
}

func TestDecodeQRWithLog(t *testing.T) {
	contents := "otpauth-migration://offline?data=SGVsbG8h"
	bm, err := qrcode.NewQRCodeWriter().EncodeWithoutHint(contents, gozxing.BarcodeFormat_QR_CODE, 256, 256)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	img := bitMatrixToImage(bm, 8)

	var log bytes.Buffer
	got, err := decodeQRWithLog(img, "png", &log)
	if err != nil {
		t.Fatalf("decodeQRWithLog: %v", err)
	}
	if got != contents {
		t.Errorf("decodeQRWithLog = %q, want %q", got, contents)
	}

	wantSize := fmt.Sprintf("image: %dx%d (png)", img.Bounds().Dx(), img.Bounds().Dy())
	if !strings.Contains(log.String(), wantSize) {
		t.Errorf("log missing image size %q: %q", wantSize, log.String())
	}
	if !strings.Contains(log.String(), `strategy "hybrid/default": OK`) {
		t.Errorf("log missing strategy result: %q", log.String())
	}
}

func TestDecodeQRWithLogFailure(t *testing.T) {
	// A solid-color image is not a QR code: every strategy must fail and be
	// reported in the log and in the returned error.
	img := image.NewGray(image.Rect(0, 0, 100, 100))

	var log bytes.Buffer
	_, err := decodeQRWithLog(img, "png", &log)
	if err == nil {
		t.Fatal("expected error for non-QR image")
	}
	if !strings.Contains(err.Error(), "failed to decode QR code (image 100x100)") {
		t.Errorf("error missing image dims: %v", err)
	}
	if !strings.Contains(log.String(), `strategy "hybrid/default": `) {
		t.Errorf("log missing per-strategy failures: %q", log.String())
	}
}

func TestCandidateDimensions(t *testing.T) {
	cases := []struct {
		raw  int
		want []int
	}{
		{96, []int{97}},     // 0 mod 4: snap up to nearest valid
		{97, []int{97}},     // already valid
		{98, []int{97}},     // 2 mod 4: snap down
		{95, []int{97, 93}}, // 3 mod 4: ambiguous, try both
		{100, []int{101}},
		{20, []int{21}}, // below minimum version
		{178, []int{177}},
	}
	for _, c := range cases {
		got := candidateDimensions(c.raw)
		if len(got) != len(c.want) {
			t.Errorf("candidateDimensions(%d) = %v, want %v", c.raw, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("candidateDimensions(%d) = %v, want %v", c.raw, got, c.want)
				break
			}
		}
	}
}

func TestDecodePureCorrected(t *testing.T) {
	var p []byte
	for i := 0; i < 24; i++ {
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

	img := bitMatrixToImage(bm, 4)
	got, err := decodePureCorrected(img, gozxing.NewHybridBinarizer)
	if err != nil {
		t.Fatalf("decodePureCorrected: %v", err)
	}
	if got != migrationURL {
		t.Errorf("decodePureCorrected = %q, want %q", got, migrationURL)
	}
}
