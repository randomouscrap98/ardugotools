package arduboy

import (
	"bytes"
	"image"
	//"image/color"
	"image/png"
	//"os"
)

const (
	GrayscaleScreenBytes = ScreenBytes * 8
)

// Convert a raw arduboy image (in arduboy format) to "regular" grayscale
// using the given black + white points, storing it in the given buffer
func RawToGrayscaleInto(raw []byte, out []byte, black uint8, white uint8) {
	for i, p := range raw {
		// Each byte in the original image is 8 vertical pixels
		x := i % ScreenWidth
		ybase := i / ScreenWidth * 8
		// Iterate over those vertical pixels
		for bit := 0; bit < 8; bit++ {
			j := x + (ybase+bit)*ScreenWidth
			if p&(1<<bit) == 0 {
				out[j] = black
			} else {
				out[j] = white
			}
		}
	}
}

// Convert a raw arduboy image (in arduboy format) to "regular" grayscale
// using the given black + white points
func RawToGrayscale(raw []byte, black uint8, white uint8) []byte {
	result := make([]byte, GrayscaleScreenBytes)
	RawToGrayscaleInto(raw, result, black, white)
	return result
}

// Convert a raw grayscale arduboy image into a PNG
func GrayscaleToPng(gray []byte) ([]byte, error) {
	img := image.NewGray(image.Rect(0, 0, ScreenWidth, ScreenHeight))
	img.Pix = gray
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
