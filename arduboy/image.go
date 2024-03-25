package arduboy

import (
	"bytes"
	"fmt"
	"io"
	//"os"

	"image"
	//"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
	"github.com/nfnt/resize"
)

const (
	GrayscaleScreenBytes = ScreenBytes * 8
)

// Convert a raw arduboy image (in arduboy format) to "regular" grayscale
// using the given black + white points, storing it in the given buffer
func RawToGrayscaleInto(raw []byte, out []byte, black uint8, white uint8) error {
	if len(raw) != ScreenBytes {
		return fmt.Errorf("Raw image not right size! Expected: %d, got: %d", ScreenBytes, len(raw))
	}
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
	return nil
}

// Convert a raw arduboy image (in arduboy format) to "regular" grayscale
// using the given black + white points
func RawToGrayscale(raw []byte, black uint8, white uint8) ([]byte, error) {
	result := make([]byte, GrayscaleScreenBytes)
	err := RawToGrayscaleInto(raw, result, black, white)
	return result, err
}

// Convert a grayscale image to a raw arduboy format
func GrayscaleToRaw(gray []byte, whiteThreshold uint8) ([]byte, error) {
	expectedSize := ScreenWidth * ScreenHeight
	if len(gray) != expectedSize {
		return nil, fmt.Errorf("Grayscale image wrong size! Expected: %d, got %d", expectedSize, len(gray))
	}
	result := make([]byte, ScreenBytes)
	for x := 0; x < ScreenWidth; x++ {
		for y := 0; y < ScreenHeight; y++ {
			if gray[x+y*ScreenWidth] >= whiteThreshold {
				result[x+(y/8)*ScreenWidth] |= 1 << (y & 7)
			}
		}
	}
	return result, nil
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

// Resize and downscale the given image into an arduboy image
// with arduboy dimensions (1 bit / weird format). The whiteThreshold is the start
// of what is considered "white". Everything else is black
func ImageToRaw(raw io.Reader, whiteThreshold uint8) ([]byte, error) {
	img, _, err := image.Decode(raw)
	if err != nil {
		return nil, err
	}
	resizedImg := resize.Resize(uint(ScreenWidth), uint(ScreenHeight), img, resize.Bilinear)
	grayImg := imaging.Grayscale(resizedImg)
	realGrayImg := make([]byte, ScreenWidth*ScreenHeight)
	for i := 0; i < ScreenWidth*ScreenHeight; i++ {
		realGrayImg[i] = grayImg.Pix[i*4]
	}
	arduimg, err := GrayscaleToRaw(realGrayImg, whiteThreshold)
	if err != nil {
		return nil, err
	}
	return arduimg, nil
}
