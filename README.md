# gotp

[![CI](https://github.com/isdmx/gotp/actions/workflows/ci.yml/badge.svg)](https://github.com/isdmx/gotp/actions/workflows/ci.yml)
[![Release](https://github.com/isdmx/gotp/actions/workflows/release.yml/badge.svg)](https://github.com/isdmx/gotp/actions/workflows/release.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/isdmx/gotp.svg)](https://pkg.go.dev/github.com/isdmx/gotp)

Extract OTP secrets from a Google Authenticator migration QR code.

`gotp` decodes the QR code shown by Google Authenticator's "Transfer accounts"
feature and prints each account's issuer, name, and Base32 secret, so you can
back them up or import them into another authenticator app.

## Install

With Go:

```sh
go install github.com/isdmx/gotp@latest
```

Or download a prebuilt binary from the
[releases page](https://github.com/isdmx/gotp/releases).

## Usage

```sh
gotp migration.png     # decode a QR code image (PNG or JPEG)
gotp -v screenshot.jpg # print per-strategy decode diagnostics
gotp --version         # print version information
```

Output:

```
Issuer: Google
Name: user@example.com
Secret: JBSWY3DPEHPK3PXP
-----------------------------------
```

## How it works

Google Authenticator's "Transfer accounts" feature encodes the selected OTP
accounts into a single QR code: an `otpauth-migration://` URL whose `data`
parameter holds a protobuf payload with each account's secret, name, and
issuer.

`gotp` decodes that QR code image, Base64-decodes the payload, parses the
protobuf, and prints each account's issuer, name, and Base32-encoded secret.

Because real-world images vary (resolution, JPEG artifacts, non-integer
scaling), the QR decoder tries several strategies: two binarizers, a corrected
grid sampler, and a scale pyramid that re-sizes the image, plus preprocessing
(grayscale, Otsu threshold, crop) to maximize reliability.

## Development

```sh
make help        # list all targets
make ci          # tidy-check + fmt-check + vet + lint + test-race
make build       # build ./bin/gotp with version metadata
make test-race   # tests with the race detector + coverage
make lint        # golangci-lint (strict config in .golangci.yml)
```

Requires Go (see `go.mod`) and
[golangci-lint](https://golangci-lint.run) v2; `make tools` installs them.

## Releasing

Releases are automated with [GoReleaser](https://goreleaser.com): pushing a
`v*` tag runs the release workflow, which cross-compiles binaries, builds
archives and checksums, generates a changelog, and publishes a GitHub Release.

```sh
git tag v0.1.0
git push origin v0.1.0
```

Build metadata (`version`, `commit`, `date`) is injected via `-ldflags` and
shown by `gotp --version`.

## License

Not yet specified. (Add a `LICENSE` file to set the project's license.)
