package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"net/url"
	"strings"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/makiuchi-d/gozxing/qrcode/decoder"
)

var (
	pureHints = map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_PURE_BARCODE: true,
	}
	tryHarderHints = map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
)

// decodeStrategy is one way to turn an image into a decoded QR text.
type decodeStrategy struct {
	name string
	fn   func(image.Image) (string, error)
}

// decodeWithBinarizer decodes a QR code from img using the given binarizer and
// decode hints.
func decodeWithBinarizer(img image.Image, binarizer func(gozxing.LuminanceSource) gozxing.Binarizer, hints map[gozxing.DecodeHintType]interface{}) (string, error) {
	bmp, err := gozxing.NewBinaryBitmap(binarizer(gozxing.NewLuminanceSourceFromImage(img)))
	if err != nil {
		return "", err
	}
	result, err := qrcode.NewQRCodeReader().Decode(bmp, hints)
	if err != nil {
		return "", err
	}
	return result.GetText(), nil
}

// strategies lists decode attempts in order, from most general to most
// specialized. gozxing's detector and its PURE_BARCODE sampler each have
// failure modes on real-world images, so we try several combinations.
func strategies() []decodeStrategy {
	hybrid := gozxing.NewHybridBinarizer
	global := gozxing.NewGlobalHistgramBinarizer
	return []decodeStrategy{
		{"hybrid/default", func(img image.Image) (string, error) {
			return decodeWithBinarizer(img, hybrid, nil)
		}},
		{"hybrid/try-harder", func(img image.Image) (string, error) {
			return decodeWithBinarizer(img, hybrid, tryHarderHints)
		}},
		{"hybrid/pure-barcode", func(img image.Image) (string, error) {
			return decodeWithBinarizer(img, hybrid, pureHints)
		}},
		{"hybrid/pure-corrected", func(img image.Image) (string, error) {
			return decodePureCorrected(img, hybrid)
		}},
		{"global/default", func(img image.Image) (string, error) {
			return decodeWithBinarizer(img, global, nil)
		}},
		{"global/pure-barcode", func(img image.Image) (string, error) {
			return decodeWithBinarizer(img, global, pureHints)
		}},
		{"global/pure-corrected", func(img image.Image) (string, error) {
			return decodePureCorrected(img, global)
		}},
		{"preprocess/default", func(img image.Image) (string, error) {
			return decodeWithBinarizer(preprocess(img, 800), hybrid, nil)
		}},
		{"preprocess/pure-barcode", func(img image.Image) (string, error) {
			return decodeWithBinarizer(preprocess(img, 800), hybrid, pureHints)
		}},
		{"preprocess/pure-corrected", func(img image.Image) (string, error) {
			return decodePureCorrected(preprocess(img, 800), hybrid)
		}},
	}
}

// scaleFactors are the relative sizes tried when direct decoding fails.
// gozxing's dimension estimate is sensitive to module size in pixels, and
// re-scaling the image shifts it off its fragile values.
var scaleFactors = []float64{0.5, 0.75, 1.25, 1.5, 1.75, 2.0, 2.5, 3.0}

// pyramidStrategies is a smaller, faster set of attempts used at each scaled
// size in the scale pyramid.
func pyramidStrategies() []decodeStrategy {
	hybrid := gozxing.NewHybridBinarizer
	return []decodeStrategy{
		{"hybrid/default", func(img image.Image) (string, error) {
			return decodeWithBinarizer(img, hybrid, nil)
		}},
		{"hybrid/pure-corrected", func(img image.Image) (string, error) {
			return decodePureCorrected(img, hybrid)
		}},
		{"preprocess/default", func(img image.Image) (string, error) {
			return decodeWithBinarizer(preprocess(img, 800), hybrid, nil)
		}},
	}
}

// decodeQR reads the text encoded in a QR code image.
func decodeQR(img image.Image) (string, error) {
	return decodeQRWithLog(img, "", nil)
}

// decodeQRWithLog is decodeQR with per-strategy diagnostics written to log
// (may be nil). format is the detected image format, for reporting only.
func decodeQRWithLog(img image.Image, format string, log io.Writer) (string, error) {
	debugf := func(format string, args ...any) {
		if log != nil {
			fmt.Fprintf(log, format+"\n", args...)
		}
	}

	b := img.Bounds()
	debugf("image: %dx%d (%s)", b.Dx(), b.Dy(), format)

	// Direct attempts at the original size.
	text, directErr := runStrategies(img, strategies(), debugf)
	if directErr == nil {
		return text, nil
	}

	// Scale pyramid: re-size the image and retry a smaller strategy set.
	for _, factor := range scaleFactors {
		resized := scaleImage(img, factor)
		// Skip very large upscales; they are slow and rarely help.
		if resized.Bounds().Dx() > 1500 || resized.Bounds().Dy() > 1500 {
			continue
		}
		debugf("scaled %.2fx -> %dx%d", factor, resized.Bounds().Dx(), resized.Bounds().Dy())
		if text, err := runStrategies(resized, pyramidStrategies(), debugf); err == nil {
			return text, nil
		}
	}

	return "", fmt.Errorf("failed to decode QR code (image %dx%d): %v",
		b.Dx(), b.Dy(), directErr)
}

// runStrategies tries each strategy in order, logging outcomes to debugf. It
// returns the decoded text on success, or a combined error on failure.
func runStrategies(img image.Image, ss []decodeStrategy, debugf func(string, ...any)) (string, error) {
	var attempts []string
	for _, s := range ss {
		text, err := s.fn(img)
		if err == nil {
			debugf("strategy %q: OK", s.name)
			return text, nil
		}
		debugf("strategy %q: %v", s.name, err)
		attempts = append(attempts, fmt.Sprintf("%s: %v", s.name, err))
	}
	return "", errors.New(strings.Join(attempts, "; "))
}

// decodePureCorrected is a PURE_BARCODE-style decode that corrects the
// estimated module count to a valid QR dimension. gozxing's own sampler
// rejects counts that are not 1 mod 4 (e.g. 96), which happens when the module
// size estimate is off by a fraction of a pixel on real-world images.
func decodePureCorrected(img image.Image, binarizer func(gozxing.LuminanceSource) gozxing.Binarizer) (string, error) {
	bmp, err := gozxing.NewBinaryBitmap(binarizer(gozxing.NewLuminanceSourceFromImage(img)))
	if err != nil {
		return "", err
	}
	black, err := bmp.GetBlackMatrix()
	if err != nil {
		return "", err
	}

	leftTop := black.GetTopLeftOnBit()
	rightBottom := black.GetBottomRightOnBit()
	if leftTop == nil || rightBottom == nil {
		return "", errors.New("no dark pixels found")
	}

	left, top := leftTop[0], leftTop[1]
	right, bottom := rightBottom[0], rightBottom[1]

	diagModuleSize := estimateModuleSize(black, leftTop)
	if diagModuleSize <= 0 {
		return "", errors.New("cannot estimate module size")
	}

	rawWidth := int(math.Round(float64(right-left+1) / diagModuleSize))
	rawHeight := int(math.Round(float64(bottom-top+1) / diagModuleSize))
	raw := rawWidth
	if rawHeight != rawWidth {
		raw = int(math.Round(float64(rawWidth+rawHeight) / 2))
	}

	dec := decoder.NewDecoder()
	var lastErr error
	for _, dim := range candidateDimensions(raw) {
		// The diagonal trace and the bounding box are independent module-size
		// estimates; when they disagree the grid can be off by a module, so
		// try both (plus the bounding-box-derived size per candidate).
		moduleSizes := []float64{
			diagModuleSize,
			float64(right-left+1) / float64(dim),
			float64(bottom-top+1) / float64(dim),
		}
		for _, ms := range moduleSizes {
			if ms <= 0 {
				continue
			}
			bits, err := samplePureBits(black, left, top, right, bottom, ms, dim)
			if err != nil {
				lastErr = err
				continue
			}
			result, err := dec.Decode(bits, nil)
			if err == nil {
				return result.GetText(), nil
			}
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no valid QR dimension")
	}
	return "", lastErr
}

// estimateModuleSize measures the module width in pixels by tracing diagonally
// from the top-left dark pixel across 7 modules (5 black/white transitions).
func estimateModuleSize(black *gozxing.BitMatrix, leftTop []int) float64 {
	width, height := black.GetWidth(), black.GetHeight()
	x, y := leftTop[0], leftTop[1]
	inBlack := true
	transitions := 0
	for x < width && y < height {
		if inBlack != black.Get(x, y) {
			transitions++
			if transitions == 5 {
				break
			}
			inBlack = !inBlack
		}
		x++
		y++
	}
	if x == width || y == height {
		return 0
	}
	return float64(x-leftTop[0]) / 7.0
}

// candidateDimensions returns valid QR dimensions near raw, nearest first. QR
// dimensions are 17 + 4*version, so always 1 mod 4 and in [21, 177].
func candidateDimensions(raw int) []int {
	seen := map[int]bool{}
	var dims []int
	for _, d := range []int{raw, raw + 1, raw - 1, raw + 2, raw - 2} {
		if d < 21 || d > 177 || d%4 != 1 || seen[d] {
			continue
		}
		seen[d] = true
		dims = append(dims, d)
	}
	return dims
}

// samplePureBits samples an axis-aligned dim x dim grid from the binarized
// matrix, mirroring gozxing's PURE_BARCODE sampling (with half-module nudge).
func samplePureBits(black *gozxing.BitMatrix, left, top, right, bottom int, moduleSize float64, dim int) (*gozxing.BitMatrix, error) {
	nudge := int(moduleSize / 2.0)
	top += nudge
	left += nudge

	nudgedTooFarRight := left + int(float64(dim-1)*moduleSize) - right
	if nudgedTooFarRight > 0 {
		if nudgedTooFarRight > nudge {
			return nil, errors.New("sample out of bounds horizontally")
		}
		left -= nudgedTooFarRight
	}
	nudgedTooFarDown := top + int(float64(dim-1)*moduleSize) - bottom
	if nudgedTooFarDown > 0 {
		if nudgedTooFarDown > nudge {
			return nil, errors.New("sample out of bounds vertically")
		}
		top -= nudgedTooFarDown
	}

	bits, err := gozxing.NewBitMatrix(dim, dim)
	if err != nil {
		return nil, err
	}
	for y := 0; y < dim; y++ {
		iOffset := top + int(float64(y)*moduleSize)
		for x := 0; x < dim; x++ {
			if black.Get(left+int(float64(x)*moduleSize), iOffset) {
				bits.Set(x, y)
			}
		}
	}
	return bits, nil
}

// extractMigrationData pulls the base64 "data" parameter out of an
// otpauth-migration URL and decodes it into the raw protobuf bytes.
func extractMigrationData(text string) ([]byte, error) {
	u, err := url.Parse(text)
	if err != nil {
		return nil, err
	}
	data := u.Query().Get("data")
	if data == "" {
		return nil, errors.New("migration URL has no data parameter")
	}
	return decodeBase64(data)
}

// decodeBase64 decodes standard Base64 with or without '=' padding.
func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}
