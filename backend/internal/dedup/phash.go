// Package dedup provides perceptual hash computation for images and video thumbnails.
package dedup

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"math"
	"os"
)

const hashSize = 8 // 8×8 = 64-bit hash

// ComputeFromFile computes the perceptual hash (pHash) of the image at the given file path.
// It returns a uint64 hash value.
func ComputeFromFile(_ context.Context, path string) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("phash open %q: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	img, _, err := image.Decode(f)
	if err != nil {
		return 0, fmt.Errorf("phash decode %q: %w", path, err)
	}

	return computePHash(img), nil
}

// computePHash implements the DCT-based perceptual hash algorithm:
//  1. Resize to 32×32 grayscale
//  2. Apply 2D DCT
//  3. Keep top-left 8×8 of DCT coefficients
//  4. Compute the mean of those 64 values
//  5. Set each bit: 1 if value ≥ mean, 0 otherwise
func computePHash(img image.Image) uint64 {	const dctSize = 32
	pixels := resizeGrayscale(img, dctSize, dctSize)
	dct := dct2D(pixels, dctSize)

	// Extract top-left hashSize × hashSize sub-matrix and compute mean
	var vals [hashSize * hashSize]float64
	var sum float64
	for y := 0; y < hashSize; y++ {
		for x := 0; x < hashSize; x++ {
			v := dct[y][x]
			vals[y*hashSize+x] = v
			sum += v
		}
	}
	mean := sum / float64(hashSize*hashSize)

	var hash uint64
	for i, v := range vals {
		if v >= mean {
			hash |= 1 << uint(i)
		}
	}
	return hash
}

// resizeGrayscale produces a w×h float64 grayscale pixel grid by nearest-neighbour
// down-sampling the source image.
func resizeGrayscale(src image.Image, w, h int) [][]float64 {
	bounds := src.Bounds()
	srcW := bounds.Max.X - bounds.Min.X
	srcH := bounds.Max.Y - bounds.Min.Y

	out := make([][]float64, h)
	for y := 0; y < h; y++ {
		out[y] = make([]float64, w)
		srcY := bounds.Min.Y + y*srcH/h
		for x := 0; x < w; x++ {
			srcX := bounds.Min.X + x*srcW/w
			r, g, b, _ := src.At(srcX, srcY).RGBA()
			// Convert to [0,255] range and compute luminance
			rf := float64(r >> 8)
			gf := float64(g >> 8)
			bf := float64(b >> 8)
			out[y][x] = 0.299*rf + 0.587*gf + 0.114*bf
		}
	}
	return out
}

// dct2D computes the 2D Discrete Cosine Transform (DCT-II) of an n×n pixel grid.
func dct2D(pixels [][]float64, n int) [][]float64 {
	// Apply DCT row-wise then column-wise (separable transform).
	tmp := make([][]float64, n)
	for i := range tmp {
		tmp[i] = dct1D(pixels[i], n)
	}

	// Transpose
	transposed := make([][]float64, n)
	for i := range transposed {
		transposed[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			transposed[i][j] = tmp[j][i]
		}
	}

	// Apply DCT column-wise
	result := make([][]float64, n)
	for i := range result {
		result[i] = dct1D(transposed[i], n)
	}

	// Transpose back
	out := make([][]float64, n)
	for i := range out {
		out[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			out[i][j] = result[j][i]
		}
	}
	return out
}

// dct1D computes the 1D DCT-II of a slice of n values.
func dct1D(vals []float64, n int) []float64 {
	out := make([]float64, n)
	scale := math.Pi / float64(n)
	for k := 0; k < n; k++ {
		var sum float64
		for i, v := range vals {
			sum += v * math.Cos(scale*float64(k)*(float64(i)+0.5))
		}
		out[k] = sum
	}
	return out
}
