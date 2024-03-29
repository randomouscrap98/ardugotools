package arduboy

import (
	"bytes"
	"fmt"
	"io"

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
	expectedRawSize := width * height / 8
	if height&7 != 0 {
		return nil, fmt.Errorf("Invalid height! Must be a multiple of 8!")
	}
	if len(raw) != expectedRawSize {
		return nil, fmt.Errorf("Raw image not right size! Expected: %d, got: %d", expectedRawSize, len(raw))
	}
	for i, p := range raw {
		// Each byte in the original image is 8 vertical pixels
		x := i % width
		ybase := i / width * 8
		// Iterate over those vertical pixels
		for bit := 0; bit < 8; bit++ {
			j := x + (ybase+bit)*width
			if p&(1<<bit) == 0 {
				result[j] = 0
			} else {
				result[j] = 1
			}
		}
	}
	return result, nil //err
}

// Convert a paletted image to a raw arduboy format
func PalettedToRaw(raw []byte, width int, height int) ([]byte, error) {
	expectedSize := width * height
	// This may not be required...
	if height&7 != 0 {
		return nil, fmt.Errorf("Invalid height! Must be a multiple of 8!")
	}
	if len(raw) != expectedSize {
		return nil, fmt.Errorf("Paletted image wrong size! Expected: %d, got %d", expectedSize, len(raw))
	}
	result := make([]byte, width*height/8)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			if raw[x+y*width] > 0 {
				result[x+(y/8)*width] |= 1 << (y & 7)
			}
		}
	}
	return result, nil
}

func PalettedToRawTitle(raw []byte) ([]byte, error) {
	return PalettedToRaw(raw, ScreenWidth, ScreenHeight)
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
		return fmt.Errorf("Unknown format: %s", format)
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

// Convert real image to paletted image, no resizing
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
	Width              int  // Width of tile (0 means use all available width)
	Height             int  // Height of tile (0 means use all available height)
	Spacing            int  // Spacing between tiles (including on edges)
	UseMask            bool // Whether to use transparency as a data mask
	SeparateHeaderMask bool // Separate the mask from the data
	AddDimensions      bool // field(default=True)
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
		return fmt.Errorf("Can't generate images with a 0-length side!")
	}
	return nil
}

// Ensure computed tile config is valid. Check returned error for nil
func (c *TileConfigComputed) ValidateForCode() error {
	if c.SpriteWidth > 255 || c.SpriteHeight > 255 {
		return fmt.Errorf("Image frames too large for code generation! Must be < 256 in both dimensions (per frame)!")
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
		config = &TileConfig{AddDimensions: true}
	}
	img, _, err := image.Decode(rawimage)
	if err != nil {
		return nil, nil, err
	}
	bounds := img.Bounds()
	imgwidth := bounds.Dx()
	imgheight := bounds.Dy()
	computed := config.Expand(imgwidth, imgheight)
	//fmt.Printf("Computed: %v", computed)
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

// Convert the given paletted image to the header data + fxdata
// (returns a tuple). Taken almost directly from https://github.com/MrBlinky/Arduboy-Python-Utilities/blob/main/image-converter.py
/*
func PalettedToCodeAndFx(paletted []byte, name string, config * TileConfig) (string, []byte, error) {
    if config == nil {
        config = TileConfig { AddDimensions: true }
    }
    spriteName := slugify.slugify(name, lowercase=False).replace("-","_")
    //img = img.convert("RGBA")
    //pixels = list(img.getdata())

    spriteWidth, spriteHeight, hframes, vframes = expand_tileconfig(config, img)

    // NOTE: images with sizes larger than uint8_t are technically invalid for the code generation,
    // BUT valid for fx generation. As such, we let them be

    spacing = config.spacing
    transparency = config.use_mask

    #create byte array for bin file
    size = (spriteHeight+7) // 8 * spriteWidth * hframes * vframes
    bytes = bytearray([spriteWidth >> 8, spriteWidth & 0xFF, spriteHeight >> 8, spriteHeight & 0xFF])
    bytes += bytearray(size + (size if transparency else 0))
    i = 4

    headerfile = io.StringIO()
    headermask = io.StringIO()  # We track the separate mask even if we don't end up using it.

    headerfile.write("constexpr uint8_t {}Width = {};\n".format(spriteName, spriteWidth))
    headerfile.write("constexpr uint8_t {}Height = {};\n".format(spriteName,spriteHeight))
    headerfile.write("\n")
    headerfile.write("constexpr uint8_t {}[] PROGMEM\n".format(spriteName,))
    headerfile.write("{\n")

    if config.add_dimensions:
        headerfile.write("  {}Width, {}Height,\n\n".format(spriteName, spriteName))

    headermask.write(f"constexpr uint8_t {spriteName}_Mask[] PROGMEM\n{{\n")

    fy = spacing
    frames = 0

    for v in range(vframes):
        fx = spacing
        for h in range(hframes):
            headerfile.write("  //Frame {}\n".format(frames))
            headermask.write("  //Mask Frame {}\n".format(frames))
            for y in range (0,spriteHeight,8):
                line = "  "
                maskline = "  "
                for x in range (0,spriteWidth):
                    b = 0
                    m  = 0
                    for p in range (0,8):
                        b = b >> 1
                        m = m >> 1
                        if (y + p) < spriteHeight: #for heights that are not a multiple of 8 pixels
                            pindex = (fy + y + p) * img.size[0] + fx + x
                            if pixels[pindex][1] > IMAGE_THRESHOLD:
                                b |= 0x80 #white pixel
                            if pixels[pindex][3] > ALPHA_THRESHOLD:
                                m |= 0x80 #opaque pixel
                            else:
                                b &= 0x7F #for transparent pixel clear possible white pixel
                    bytes[i] = b
                    i += 1
                    line += "0x{:02X}, ".format(b)
                    maskline += "0x{:02X}, ".format(m)
                    if transparency:
                        # Must always interleave bytes of fx data, regardless of 'separate mask'
                        bytes[i] = m
                        i += 1
                        # But you interleave header only if not separate set!
                        if not config.separate_header_mask:
                            line += "0x{:02X}, ".format(m)
                lastline = (v+1 == vframes) and (h+1 == hframes) and (y+8 >= spriteHeight)
                if lastline:
                    line = line [:-2]
                    maskline = maskline[:-2]
                headerfile.write(line + "\n")
                headermask.write(maskline + "\n")
            if not lastline:
                headerfile.write("\n")
                headermask.write("\n")
            frames += 1
            fx += spriteWidth + spacing
        fy += spriteHeight + spacing

    headerfile.write("};\n")
    headermask.write("};\n")

    # We've been tracking mask separately. Go ahead and add the separate mask to the final data
    # if that's the exact config desired.
    if transparency and config.separate_header_mask:
        headermask.seek(0)
        headerfile.write("\n" + headermask.read())
        # bytes += maskbytes # Add maskbytes to end of byte array

    headerfile.seek(0)

    return headerfile.read(),bytes

  }

*/
