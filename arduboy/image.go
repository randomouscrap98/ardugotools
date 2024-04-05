package arduboy

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
	"github.com/nfnt/resize"
	"golang.org/x/image/bmp"
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
func RawToPaletted(raw []byte, width int, height int) ([]byte, error) {
	result := make([]byte, width*height)
	// Drop bottom 3 bits; height is / 8 (and we want to drop the unused bits)
	expectedRawSize := width * (height >> 3)
	if len(raw) != expectedRawSize {
		return nil, fmt.Errorf("raw image not right size! Expected: %d, got: %d", expectedRawSize, len(raw))
	}
	for i, p := range raw {
		// Each byte in the original image is 8 vertical pixels
		x := i % width
		ybase := i / width * 8
		// Iterate over those vertical pixels
		for bit := 0; bit < 8; bit++ {
			j := x + (ybase+bit)*width
			// For heights that aren't multiple of 8, the bits technically go outside it
			if j < len(result) {
				if p&(1<<bit) == 0 {
					result[j] = 0
				} else {
					result[j] = 1
				}
			}
		}
	}
	return result, nil //err
}

// Convert a paletted image to a raw arduboy format
func PalettedToRaw(paletted []byte, width int, height int) ([]byte, []byte, error) {
	expectedSize := width * height
	if len(paletted) != expectedSize {
		return nil, nil, fmt.Errorf("paletted image wrong size! Expected: %d, got %d", expectedSize, len(paletted))
	}
	rheight := height >> 3
	// If not a multiple of 8, height needs to be larger
	if height&7 > 0 {
		rheight++
	}
	result := make([]byte, width*rheight)
	mask := make([]byte, width*rheight)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			rpos := x + (y/8)*width
			rbit := uint8(1 << (y & 7))
			pix := paletted[x+y*width]
			if pix < 2 {
				// Always set mask if pixel is opaque (0 and 1)
				mask[rpos] |= rbit
				if pix == 1 {
					// Only set result if it's specifically white
					result[rpos] |= rbit
				}
			}
		}
	}
	return result, mask, nil
}

func PalettedToRawTitle(raw []byte) ([]byte, error) {
	result, _, err := PalettedToRaw(raw, ScreenWidth, ScreenHeight)
	return result, err
}

func RawToPalettedTitle(raw []byte) ([]byte, error) {
	return RawToPaletted(raw, ScreenWidth, ScreenHeight)
}

// Convert a paletted raw to an image of the given format. Possible values are
// gif, png, bmp, jpg. Transparency is possible if the right colors are chosen,
// but only two colors are allowed
func PalettedToImage(raw []byte, width int, height int, black color.Color, white color.Color, format string, writer io.Writer) error {
	palette := color.Palette{black, white, color.Transparent, color.RGBA{R: 255}}
	img := image.NewPaletted(image.Rect(0, 0, width, height), palette)
	img.Pix = raw
	//var buf bytes.Buffer
	var err error
	if format == "gif" {
		err = gif.Encode(writer, img, &gif.Options{
			NumColors: 4, Quantizer: nil, Drawer: nil,
		})
	} else if format == "png" {
		err = png.Encode(writer, img)
	} else if format == "jpg" {
		err = jpeg.Encode(writer, img, nil)
	} else if format == "bmp" {
		err = bmp.Encode(writer, img)
	} else {
		return fmt.Errorf("unknown format: %s", format)
	}
	if err != nil {
		return err
	}
	return nil
}

// Convert a paletted image to a real image in the given format. Don't bother with
// a writer, since titles are so small
func PalettedToImageTitleBW(raw []byte, format string) ([]byte, error) {
	var buf bytes.Buffer
	err := PalettedToImage(raw, ScreenWidth, ScreenHeight, color.Black, color.White, format, &buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Convert real image to paletted image, no resizing. Returns paletted
// blob and width/height of image
func ImageToPaletted(img image.Image, whiteThreshold uint8, alphaThreshold uint8) ([]byte, int, int) {
	grayImg := imaging.Grayscale(img)
	width := grayImg.Rect.Dx()
	height := grayImg.Rect.Dy()
	paletteImg := make([]byte, width*height)
	for i := 0; i < width*height; i++ {
		if grayImg.Pix[i*4+3] < alphaThreshold {
			paletteImg[i] = 2
		} else if grayImg.Pix[i*4] >= whiteThreshold {
			paletteImg[i] = 1
		}
	}
	return paletteImg, grayImg.Rect.Dx(), grayImg.Rect.Dy()
}

// If you haven't already decoded the image, we can do that for you
func RawImageToPaletted(raw io.Reader, whiteThreshold uint8, alphaThreshold uint8) ([]byte, int, int, error) {
	img, _, err := image.Decode(raw)
	if err != nil {
		return nil, 0, 0, err
	}
	res, w, h := ImageToPaletted(img, whiteThreshold, alphaThreshold)
	return res, w, h, nil
}

// Resize and downscale the given image into a paletted image
// with arduboy dimensions. The whiteThreshold is the start
// of what is considered "white". Everything else is black
func RawImageToPalettedTitle(raw io.Reader, whiteThreshold uint8) ([]byte, error) {
	img, _, err := image.Decode(raw)
	if err != nil {
		return nil, err
	}
	resizedImg := resize.Resize(uint(ScreenWidth), uint(ScreenHeight), img, resize.Bilinear)
	res, _, _ := ImageToPaletted(resizedImg, whiteThreshold, 0)
	return res, nil
}

// Configuration for tile / code generation
type TileConfig struct {
	Width         int    // Width of tile (0 means use all available width)
	Height        int    // Height of tile (0 means use all available height)
	Spacing       int    // Spacing between tiles (including on edges)
	UseMask       bool   // Whether to use transparency as a data mask
	SeparateMask  bool   // Separate the mask from the data
	NoDimensions  bool   // Don't output dimension variables in data
	NoPreamble    bool   // Don't generate the preamble (includes, etc)
	WindowsFormat bool   // Windows newlines (\r\n)
	Name          string // Name of the sprite variables to generate
}

// Extra computed fields when we know more about the image we're applying
// the tile config to
type TileConfigComputed struct {
	SpriteWidth  int // Calculated width of each sprite
	SpriteHeight int // Calculated height of each sprite
	HFrames      int // How many tiles across
	VFrames      int // How many tiles vertical
	StartX       int // Where to start reading tiles within the image
	StartY       int // Where to start reading tiles within the image
	StrideX      int // How far to move through the image to find the next tile
	StrideY      int // How far to move through the image to find the next tile
}

// Calculate individaul sprite width, height, horizontal count, and vertical count
func (t *TileConfig) Expand(width int, height int) *TileConfigComputed {
	var result TileConfigComputed

	if t.Width > 0 {
		// Known width, calculate HFrames
		result.SpriteWidth = t.Width
		result.HFrames = (width - t.Spacing) / (result.SpriteWidth + t.Spacing)
	} else {
		// Unknown width, use whole thing. ONly one hframe
		result.SpriteWidth = width - 2*t.Spacing
		result.HFrames = 1
	}
	if t.Height > 0 {
		// Known height, calculate VFrames
		result.SpriteHeight = t.Height
		result.VFrames = (height - t.Spacing) / (result.SpriteHeight + t.Spacing)
	} else {
		// Unknown height, use whole thing. ONly one vframe
		result.SpriteHeight = height - 2*t.Spacing
		result.VFrames = 1
	}

	result.StartX = t.Spacing
	result.StartY = t.Spacing
	result.StrideX = t.Spacing + result.SpriteWidth
	result.StrideY = t.Spacing + result.SpriteHeight

	return &result
}

func (c *TileConfigComputed) ValidateGeneral() error {
	if c.SpriteWidth <= 0 || c.SpriteHeight <= 0 {
		return fmt.Errorf("can't generate images with a 0-length side")
	}
	return nil
}

// Ensure computed tile config is valid. Check returned error for nil
func (c *TileConfigComputed) ValidateForCode() error {
	if c.SpriteWidth > 255 || c.SpriteHeight > 255 {
		return fmt.Errorf("image frames too large for code generation! Must be < 256 in both dimensions (per frame)")
	}
	return c.ValidateGeneral()
}

// Ensure computed tile config is valid for writing to fx.
func (c *TileConfigComputed) ValidateForFx() error {
	return c.ValidateGeneral()
}

// Split the given image into linear tiles based on the given tile config. returns the
// array of tile images, each in NRGBA format
func SplitImageToTiles(rawimage io.Reader, config *TileConfig) ([]*image.NRGBA, *TileConfigComputed, error) {
	if config == nil {
		config = &TileConfig{}
	}
	img, _, err := image.Decode(rawimage)
	if err != nil {
		return nil, nil, err
	}
	bounds := img.Bounds()
	imgwidth := bounds.Dx()
	imgheight := bounds.Dy()
	computed := config.Expand(imgwidth, imgheight)
	err = computed.ValidateGeneral()
	if err != nil {
		return nil, nil, err
	}
	results := make([]*image.NRGBA, 0)
	expectedTileLength := 4 * computed.SpriteWidth * computed.SpriteHeight
	// Now very carefully crop everything...
	for yf := 0; yf < computed.VFrames; yf++ {
		for xf := 0; xf < computed.HFrames; xf++ {
			sprite := imaging.Crop(img, image.Rect(
				computed.StartX+computed.StrideX*xf,
				computed.StartY+computed.StrideY*yf,
				computed.StartX+computed.StrideX*xf+computed.SpriteWidth,
				computed.StartY+computed.StrideY*yf+computed.SpriteHeight,
			))
			results = append(results, sprite)
			if len(sprite.Pix) != expectedTileLength {
				return nil, nil, fmt.Errorf(
					"PROGRAM ERROR: cropped tile (%d,%d) not right size! Expected: %d, got: %d",
					xf, yf, expectedTileLength, len(sprite.Pix))
			}
		}
	}
	return results, computed, nil
}

// Convert the given paletted image to the header data. Taken almost directly from
// https://github.com/MrBlinky/Arduboy-Python-Utilities/blob/main/image-converter.py
// THIS FUNCTION CAN BE MEMORY INTENSIVE! The entire code file is buffered in memory!
func PalettedToCode(ptiles [][]byte, config *TileConfig, computed *TileConfigComputed) (string, error) {
	if config == nil {
		config = &TileConfig{}
	}
	spritename := config.Name
	if spritename == "" {
		spritename = "Spritesheet"
	}

	var headerfile strings.Builder
	var headermask strings.Builder // We track the separate mask even if we don't end up using it.

	if !config.NoPreamble {
		headerfile.WriteString("#pragma once\n\n")
		headerfile.WriteString("#include <stdint.h>\n")
		headerfile.WriteString("#include <avr/pgmspace.h>\n\n")
	}

	headerfile.WriteString(fmt.Sprintf("constexpr uint8_t %sWidth = %d;\n", spritename, computed.SpriteWidth))
	headerfile.WriteString(fmt.Sprintf("constexpr uint8_t %sHeight = %d;\n", spritename, computed.SpriteHeight))
	headerfile.WriteString("\n")
	headerfile.WriteString(fmt.Sprintf("constexpr uint8_t %s[] PROGMEM\n{", spritename))

	if !config.NoDimensions {
		headerfile.WriteString(fmt.Sprintf("\n  %sWidth, %sHeight,\n", spritename, spritename))
	}

	headermask.WriteString(fmt.Sprintf("constexpr uint8_t %s_Mask[] PROGMEM\n{", spritename))

	for i, ptile := range ptiles {
		headerfile.WriteString(fmt.Sprintf("\n  // Frame %d", i))
		headermask.WriteString(fmt.Sprintf("\n  // Mask Frame %d", i))
		raw, mask, err := PalettedToRaw(ptile, computed.SpriteWidth, computed.SpriteHeight)
		if err != nil {
			return "", err
		}
		for j := 0; j < len(raw); j++ {
			// Put each row of the tile on a different line
			if j%computed.SpriteWidth == 0 {
				headerfile.WriteString("\n  ")
				headermask.WriteString("\n  ")
			}
			headerfile.WriteString(fmt.Sprintf("0x%02X", raw[j]))
			headermask.WriteString(fmt.Sprintf("0x%02X", mask[j]))
			// Interleave mask bytes into header
			if config.UseMask && !config.SeparateMask {
				headerfile.WriteString(fmt.Sprintf(", 0x%02X", mask[j]))
			}
			// Wasteful computation to not put the last comma on the very last iteration
			if !(i == len(ptiles)-1 && j == len(raw)-1) {
				headerfile.WriteString(", ")
				headermask.WriteString(", ")
			}
		}
	}

	headerfile.WriteString("\n};\n")
	headermask.WriteString("\n};\n")

	// We've been tracking mask either separately or interleaved. If separate, Go
	// ahead and add the separate mask to the final data
	if config.UseMask && config.SeparateMask {
		headerfile.WriteString("\n")
		headerfile.WriteString(headermask.String())
	}

	result := headerfile.String()

	if config.WindowsFormat {
		result = strings.ReplaceAll(result, "\n", "\r\n")
	}

	return result, nil
}
