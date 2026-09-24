package main

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/draw"
)

// toGray converts img to an 8-bit grayscale image.
func toGray(img image.Image) *image.Gray {
	b := img.Bounds()
	g := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := range b.Dy() {
		for x := range b.Dx() {
			g.Set(x, y, color.GrayModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)))
		}
	}
	return g
}

// otsuThreshold computes the Otsu binarization threshold for a grayscale image.
func otsuThreshold(g *image.Gray) uint8 {
	b := g.Bounds()
	var hist [256]int
	for y := range b.Dy() {
		for x := range b.Dx() {
			hist[g.GrayAt(x, y).Y]++
		}
	}
	total := b.Dx() * b.Dy()
	sum := 0
	for i := range 256 {
		sum += i * hist[i]
	}
	sumB := 0
	wB := 0
	var maxVar float64
	threshold := 0
	for t := range 256 {
		wB += hist[t]
		if wB == 0 {
			continue
		}
		wF := total - wB
		if wF == 0 {
			break
		}
		sumB += t * hist[t]
		mB := float64(sumB) / float64(wB)
		mF := float64(sum-sumB) / float64(wF)
		between := float64(wB) * float64(wF) * (mB - mF) * (mB - mF)
		if between >= maxVar {
			maxVar = between
			threshold = t
		}
	}
	return uint8(threshold)
}

// binarize returns a pure black/white image using the given threshold.
func binarize(g *image.Gray, threshold uint8) *image.Gray {
	b := g.Bounds()
	out := image.NewGray(b)
	for y := range b.Dy() {
		for x := range b.Dx() {
			c := color.Gray{Y: 255}
			if g.GrayAt(x, y).Y <= threshold {
				c.Y = 0
			}
			out.SetGray(x, y, c)
		}
	}
	return out
}

// cropToDark crops to the bounding box of dark pixels plus margin pixels of
// padding, which re-adds a quiet zone around the code.
func cropToDark(bin *image.Gray, margin int) *image.Gray {
	b := bin.Bounds()
	minX, minY := b.Dx(), b.Dy()
	maxX, maxY := -1, -1
	for y := range b.Dy() {
		for x := range b.Dx() {
			if bin.GrayAt(x, y).Y != 0 {
				continue
			}
			minX = min(minX, x)
			maxX = max(maxX, x)
			minY = min(minY, y)
			maxY = max(maxY, y)
		}
	}
	if maxX < 0 {
		return bin
	}
	minX = max(0, minX-margin)
	minY = max(0, minY-margin)
	maxX = min(b.Dx()-1, maxX+margin)
	maxY = min(b.Dy()-1, maxY+margin)

	out := image.NewGray(image.Rect(0, 0, maxX-minX+1, maxY-minY+1))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			out.SetGray(x-minX, y-minY, bin.GrayAt(x, y))
		}
	}
	return out
}

// scaleNearest upscales img by an integer factor using nearest-neighbor
// interpolation, which preserves the sharp edges of an already-binarized image.
func scaleNearest(img image.Image, factor int) *image.Gray {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewGray(image.Rect(0, 0, w*factor, h*factor))
	for y := range h {
		for x := range w {
			c := color.GrayModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.Gray)
			for dy := range factor {
				for dx := range factor {
					out.SetGray(x*factor+dx, y*factor+dy, c)
				}
			}
		}
	}
	return out
}

// minPreprocessSize is the minimum short-side dimension the preprocessed image
// is upscaled to before decoding.
const minPreprocessSize = 800

// preprocess cleans up a QR code image for decoding: grayscale, Otsu
// threshold, crop to the QR region, and upscale to at least minPreprocessSize
// pixels.
func preprocess(img image.Image) *image.Gray {
	g := toGray(img)
	bin := binarize(g, otsuThreshold(g))
	cropped := cropToDark(bin, 16)
	short := min(cropped.Bounds().Dx(), cropped.Bounds().Dy())
	if short > 0 && short < minPreprocessSize {
		return scaleNearest(cropped, (minPreprocessSize+short-1)/short)
	}
	return cropped
}

// scaleImage resizes img by factor using smooth interpolation. Unlike
// nearest-neighbor scaling, this changes the module boundaries enough to shift
// gozxing's dimension estimate off of its fragile values.
func scaleImage(img image.Image, factor float64) *image.RGBA {
	b := img.Bounds()
	w := max(1, int(math.Round(float64(b.Dx())*factor)))
	h := max(1, int(math.Round(float64(b.Dy())*factor)))
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}
