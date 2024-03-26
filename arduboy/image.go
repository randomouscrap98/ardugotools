package arduboy

import (
	"bytes"
	"fmt"
	"io"

	//"os"

	"image"
	"image/color"
	"image/gif"
	_ "image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
	"github.com/nfnt/resize"
	"golang.org/x/image/bmp"
)

const (
	GrayscaleScreenBytes = ScreenBytes * 8
)

// Convert a raw arduboy image (in arduboy format) to "regular" grayscale
// using the given black + white points, storing it in the given buffer
// func RawToGrayscaleInto(raw []byte, out []byte, black uint8, white uint8) error {
// 	if len(raw) != ScreenBytes {
// 		return fmt.Errorf("Raw image not right size! Expected: %d, got: %d", ScreenBytes, len(raw))
// 	}
// 	for i, p := range raw {
// 		// Each byte in the original image is 8 vertical pixels
// 		x := i % ScreenWidth
// 		ybase := i / ScreenWidth * 8
// 		// Iterate over those vertical pixels
// 		for bit := 0; bit < 8; bit++ {
// 			j := x + (ybase+bit)*ScreenWidth
// 			if p&(1<<bit) == 0 {
// 				out[j] = black
// 			} else {
// 				out[j] = white
// 			}
// 		}
// 	}
// 	return nil
// }

// Convert a raw arduboy image (in arduboy format) to "regular" grayscale
// using the given black + white points
func RawToPaletted(raw []byte) ([]byte, error) {
	result := make([]byte, GrayscaleScreenBytes)
	//err := RawToGrayscaleInto(raw, result, black, white)
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
			if raw[x+y*ScreenWidth] > 0 { //>= whiteThreshold {
				result[x+(y/8)*ScreenWidth] |= 1 << (y & 7)
			}
		}
	}
	return result, nil
}

// Convert a paletted raw to an image of the given format. Possible values are
// gif, png, bmp
func PalettedToImage(raw []byte, black color.Color, white color.Color, format string) ([]byte, error) {
	palette := color.Palette{
		color.Black,
		color.White,
	}
	img := image.NewPaletted(image.Rect(0, 0, ScreenWidth, ScreenHeight), palette)
	var buf bytes.Buffer
	var err error
	if format == "gif" {
		err = gif.Encode(&buf, img, &gif.Options{
			NumColors: 2, Quantizer: nil, Drawer: nil,
		})
	} else if format == "png" {
		err = png.Encode(&buf, img)
	} else if format == "bmp" {
		err = bmp.Encode(&buf, img)
	} else {
		return nil, fmt.Errorf("Unknown format: %s", format)
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
	//img := image.PalettedImage(image.Rect(0, 0, ScreenWidth, ScreenHeight))
}

func PalettedToImageBW(raw []byte, format string) ([]byte, error) {
	return PalettedToImage(raw, color.Black, color.White, format)
}

// // Convert a raw grayscale arduboy image into a PNG
// func GrayscaleToPng(gray []byte) ([]byte, error) {
// 	img := image.NewGray(image.Rect(0, 0, ScreenWidth, ScreenHeight))
// 	img.Pix = gray
// 	var buf bytes.Buffer
// 	err := png.Encode(&buf, img)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return buf.Bytes(), nil
// }
//
// func GrayscaleToGif(gray []byte) ([]byte, error) {
// 	//img := image.NewGray(image.Rect(0, 0, ScreenWidth, ScreenHeight))
//   palette := color.Palette{
// 		color.Black,
// 		color.White,
// 	}
// 	img := image.NewPaletted(image.Rect(0, 0, ScreenWidth, ScreenHeight), palette)
// 	//img := image.NewPaletted(image.Rect(0, 0, 100, 100), palette)
// 	for i := 0; i < len(gray); i++ {
//     img.Pix[i] = gray[i]
// 		img.Pix[i*4] = gray[i]
// 		img.Pix[i*4+1] = gray[i]
// 		img.Pix[i*4+2] = gray[i]
// 		img.Pix[i*4+3] = 255
// 	}
// 	var buf bytes.Buffer
// 	//err := bmp.Encode(&buf, img)
// 	err := gif.Encode(&buf, img, &gif.Options{
// 		NumColors: 2, Quantizer: nil, Drawer: nil,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	return buf.Bytes(), nil
// }
//

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
	paletteImg := make([]byte, ScreenWidth*ScreenHeight)
	for i := 0; i < ScreenWidth*ScreenHeight; i++ {
		if grayImg.Pix[i*4] >= whiteThreshold {
			paletteImg[i] = 1
		}
		//grayImg.Pix[i*4]
	}
	arduimg, err := PalettedToRaw(paletteImg) //GrayscaleToRaw(realGrayImg, whiteThreshold)
	if err != nil {
		return nil, err
	}
	return arduimg, nil
}
