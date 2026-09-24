// Command gotp extracts OTP secrets from a Google Authenticator migration
// QR code.
package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"

	"github.com/isdmx/gotp/internal/version"
)

const helpText = `gotp extracts OTP secrets from a Google Authenticator migration QR code.

When you use Google Authenticator's "Transfer accounts" feature, it encodes all
of your OTP accounts into a single QR code. That code is an otpauth-migration://
URL whose "data" parameter holds a protobuf payload with each account's secret,
name, and issuer. This tool reads a QR code image, decodes that payload, and
prints the accounts so you can back them up or move them to another
authenticator app.

Usage:
  gotp [options] <image-path>

Arguments:
  <image-path>    Path to a QR code image (PNG or JPEG) containing a
                  Google Authenticator migration code.

Options:
  -v, --verbose   Print per-strategy decode diagnostics to stderr.
  -V, --version   Print version information and exit.
  -h, --help      Show this help message and exit.

Output:
  For each account it prints the issuer, name, and secret (Base32 encoded):

    Issuer: Google
    Name: user@example.com
    Secret: JBSWY3DPEHPK3PXP
    -----------------------------------

Examples:
  gotp migration.png
  gotp -v screenshot.jpg
`

func main() {
	verbose := false
	var args []string
	for _, a := range os.Args[1:] {
		switch a {
		case "-h", "--help":
			fmt.Print(helpText)
			os.Exit(0)
		case "-V", "--version":
			fmt.Println(version.String())
			os.Exit(0)
		case "-v", "--verbose":
			verbose = true
		default:
			args = append(args, a)
		}
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: gotp [-v] <image-path>")
		fmt.Fprintln(os.Stderr, "Run 'gotp --help' for details.")
		os.Exit(2)
	}

	var diag io.Writer
	if verbose {
		diag = os.Stderr
	}
	if err := run(args[0], os.Stdout, diag); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run decodes a QR code image containing a Google Authenticator migration
// URL and writes the extracted OTP parameters to out. Diagnostics are written
// to diag (may be nil).
func run(path string, out, diag io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer func() { _ = f.Close() }()

	img, format, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	text, err := decodeQRWithLog(img, format, diag)
	if err != nil {
		return fmt.Errorf("decode QR code: %w", err)
	}

	data, err := extractMigrationData(text)
	if err != nil {
		return fmt.Errorf("extract migration data: %w", err)
	}

	payload, err := parseMigrationPayload(data)
	if err != nil {
		return fmt.Errorf("parse migration payload: %w", err)
	}

	if _, err := fmt.Fprint(out, formatOTPCodes(payload)); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
