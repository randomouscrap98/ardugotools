package arduboy

import (
	"bytes"
	"fmt"
	"io"

	//"os"

	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
	"github.com/nfnt/resize"
	"golang.org/x/image/bmp"
)

const (
	GrayscaleScreenBytes = ScreenBytes * 8
)

func Uint32ToColor(c uint32) color.Color {
	red := uint8((c >> 24) & 0xFF)
	green := uint8((c >> 16) & 0xFF)
	blue := uint8((c >> 8) & 0xFF)
	alpha := uint8(c & 0xFF)
	return color.RGBA{R: red, G: green, B: blue, A: alpha}
}

// Convert a raw arduboy image (in arduboy format) to "regular" grayscale
// using the given black + white points
func RawToPaletted(raw []byte) ([]byte, error) {
	result := make([]byte, GrayscaleScreenBytes)
	if len(raw) != ScreenBytes {
		return nil, fmt.Errorf("Raw image not right size! Expected: %d, got: %d", ScreenBytes, len(raw))
	}
	for i, p := range raw {
		// Each byte in the original image is 8 vertical pixels
		x := i % ScreenWidth
		ybase := i / ScreenWidth * 8
		// Iterate over those vertical pixels
		for bit := 0; bit < 8; bit++ {
			j := x + (ybase+bit)*ScreenWidth
			if p&(1<<bit) == 0 {
				result[j] = 0
			} else {
				result[j] = 1
			}
		}
	}
	//return nil
	return result, nil //err
}

// Convert a paletted image to a raw arduboy format
func PalettedToRaw(raw []byte) ([]byte, error) {
	expectedSize := ScreenWidth * ScreenHeight
	if len(raw) != expectedSize {
		return nil, fmt.Errorf("Paletted image wrong size! Expected: %d, got %d", expectedSize, len(raw))
	}
	result := make([]byte, ScreenBytes)
	for x := 0; x < ScreenWidth; x++ {
		for y := 0; y < ScreenHeight; y++ {
			if raw[x+y*ScreenWidth] > 0 {
				result[x+(y/8)*ScreenWidth] |= 1 << (y & 7)
			}
		}
	}
	return result, nil
}

// Convert a paletted raw to an image of the given format. Possible values are
// gif, png, bmp
func PalettedToImage(raw []byte, black color.Color, white color.Color, format string) ([]byte, error) {
	palette := color.Palette{black, white}
	img := image.NewPaletted(image.Rect(0, 0, ScreenWidth, ScreenHeight), palette)
	img.Pix = raw
	var buf bytes.Buffer
	var err error
	if format == "gif" {
		err = gif.Encode(&buf, img, &gif.Options{
			NumColors: 2, Quantizer: nil, Drawer: nil,
		})
	} else if format == "png" {
		err = png.Encode(&buf, img)
	} else if format == "jpg" {
		err = jpeg.Encode(&buf, img, nil)
	} else if format == "bmp" {
		err = bmp.Encode(&buf, img)
	} else {
		return nil, fmt.Errorf("Unknown format: %s", format)
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func PalettedToImageBW(raw []byte, format string) ([]byte, error) {
	return PalettedToImage(raw, color.Black, color.White, format)
}

// Resize and downscale the given image into a paletted image
// with arduboy dimensions. The whiteThreshold is the start
// of what is considered "white". Everything else is black
func ImageToPaletted(raw io.Reader, whiteThreshold uint8) ([]byte, error) {
	img, _, err := image.Decode(raw)
	if err != nil {
		return nil, err
	}
	resizedImg := resize.Resize(uint(ScreenWidth), uint(ScreenHeight), img, resize.Bilinear)
	grayImg := imaging.Grayscale(resizedImg)
	paletteImg := make([]byte, ScreenWidth*ScreenHeight)
	for i := 0; i < ScreenWidth*ScreenHeight; i++ {
		if grayImg.Pix[i*4] >= whiteThreshold {
			paletteImg[i] = 1
		}
	}
	return paletteImg, nil
	// arduimg, err := PalettedToRaw(paletteImg)
	// if err != nil {
	// 	return nil, err
	// }
	// return arduimg, nil
}
