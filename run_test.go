package main

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func TestRunEndToEnd(t *testing.T) {
	// Build a migration URL from a real payload.
	migrationURL := "otpauth-migration://offline?data=" +
		base64.StdEncoding.EncodeToString(buildTestPayload())

	// Render it as a QR code PNG file, like the one Google Authenticator shows.
	bm, err := qrcode.NewQRCodeWriter().EncodeWithoutHint(
		migrationURL, gozxing.BarcodeFormat_QR_CODE, 512, 512)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	path := filepath.Join(t.TempDir(), "migration.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := png.Encode(f, bitMatrixToImage(bm, 8)); err != nil {
		f.Close()
		t.Fatalf("png encode: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	var out bytes.Buffer
	if err := run(path, &out, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := "Issuer: Google\n" +
		"Name: user@example.com\n" +
		"Secret: JBSWY3DPEE\n" +
		"-----------------------------------\n"
	if out.String() != want {
		t.Errorf("run output = %q, want %q", out.String(), want)
	}
}
