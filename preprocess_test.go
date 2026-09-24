package main

import (
	"image"
	"image/color"
	"testing"
)

func TestOtsuThresholdBimodal(t *testing.T) {
	g := image.NewGray(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if x < 50 {
				g.SetGray(x, y, color.Gray{Y: 30})
			} else {
				g.SetGray(x, y, color.Gray{Y: 220})
			}
		}
	}
	bin := binarize(g, otsuThreshold(g))
	if got := bin.GrayAt(0, 0).Y; got != 0 {
		t.Errorf("dark pixel = %d, want 0", got)
	}
	if got := bin.GrayAt(60, 0).Y; got != 255 {
		t.Errorf("light pixel = %d, want 255", got)
	}
}

func TestBinarize(t *testing.T) {
	g := image.NewGray(image.Rect(0, 0, 2, 1))
	g.SetGray(0, 0, color.Gray{Y: 10})  // dark -> black
	g.SetGray(1, 0, color.Gray{Y: 200}) // light -> white

	bin := binarize(g, 128)
	if got := bin.GrayAt(0, 0).Y; got != 0 {
		t.Errorf("dark pixel = %d, want 0", got)
	}
	if got := bin.GrayAt(1, 0).Y; got != 255 {
		t.Errorf("light pixel = %d, want 255", got)
	}
}

func TestCropToDark(t *testing.T) {
	bin := image.NewGray(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			bin.SetGray(x, y, color.Gray{Y: 255})
		}
	}
	for y := 10; y <= 20; y++ {
		for x := 10; x <= 20; x++ {
			bin.SetGray(x, y, color.Gray{Y: 0})
		}
	}

	cropped := cropToDark(bin, 5)
	b := cropped.Bounds()
	if b.Dx() != 21 || b.Dy() != 21 {
		t.Errorf("crop bounds = %v, want 21x21 (11 dark + 2*5 margin)", b)
	}
}

func TestScaleImage(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 100, 200))
	dst := scaleImage(src, 2.0)
	if dst.Bounds().Dx() != 200 || dst.Bounds().Dy() != 400 {
		t.Errorf("scaleImage(2.0) = %v, want 200x400", dst.Bounds())
	}
}

func TestScaleNearest(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 2, 1))
	src.SetGray(0, 0, color.Gray{Y: 0})
	src.SetGray(1, 0, color.Gray{Y: 255})

	dst := scaleNearest(src, 2)
	if dst.Bounds().Dx() != 4 || dst.Bounds().Dy() != 2 {
		t.Fatalf("scaleNearest(2) bounds = %v, want 4x2", dst.Bounds())
	}
	if got := dst.GrayAt(0, 0).Y; got != 0 {
		t.Errorf("pixel (0,0) = %d, want 0", got)
	}
	if got := dst.GrayAt(2, 0).Y; got != 255 {
		t.Errorf("pixel (2,0) = %d, want 255", got)
	}
}
