package arduboy

// NOTE: this file should not care about the format of the data or where it
// comes from, it should only be given the already-parsed data. As such,
// DON'T include things like the toml or json libararies!
import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

const (
	FxDevExpectedFlashCapacity = 1 << 24
)

type FxDataImageConfig struct {
	Width          int   // Width of tile (0 means use all available width)
	Height         int   // Height of tile (0 means use all available height)
	Spacing        int   // Spacing between tiles (including on edges)
	UseMask        bool  // Whether to use transparency as a data mask
	Threshold      uint8 // The upper bound for black pixels
	AlphaThreshold uint8 // The upper bound for alpha threshold
}

func (i *FxDataImageConfig) ReasonableDefaults() {
	if i.AlphaThreshold == 0 {
		i.AlphaThreshold = 10
	}
	if i.Threshold == 0 {
		i.Threshold = 100
	}
}

// A single field put into the fx data blob. May generate multiple
// fields based on configuration
type FxDataField struct {
	//Type string
	Data string
	//Delimeter []byte
	Format string
	Image  *FxDataImageConfig
}

func (d *FxDataField) ReasonableDefaults() {
	if d.Format == "" {
		d.Format = "file"
	}
	if d.Image == nil {
		d.Image = &FxDataImageConfig{}
	}
	d.Image.ReasonableDefaults()
}

// All fields requested in the entire fx data blob. It must always
// be explicit
type FxData struct {
	Data map[string]*FxDataField
	Save map[string]*FxDataField
}

// Parse a single FX field, regardless of where it's supposed to go, and
// output the results to both the header and the data writer. Accepts the
// "current location" within the data writer, and should return the updated position
// within the data writer
func ParseFxField(name string, field *FxDataField, header io.Writer, data io.Writer,
	position int) (int, error) {
	if field == nil {
		return 0, fmt.Errorf("passed null 'field' in ParseFxField: %s", name)
	}
	truelength := 0
	onebyte := make([]byte, 1)
	// Preemptively write the header field now, and later we'll write some other junk
	io.WriteString(header, MakeFxHeaderAddress(name, position))
	switch strings.ToLower(field.Format) {
	case "file":
		// File is always raw: copy it directly to the output
		file, err := os.Open(field.Data)
		if err != nil {
			return 0, err
		}
		defer file.Close()
		copylen, err := io.Copy(data, file)
		if err != nil {
			return 0, err
		}
		log.Printf("Copied raw file %s to fxdata (%d bytes)\n", field.Data, copylen)
		truelength = int(copylen)
	case "image":
		// Image is a bit more special: it needs to have a lot done to it, but
		// is eventually written almost as-is... kinda..
		file, err := os.Open(field.Data)
		if err != nil {
			return 0, err
		}
		defer file.Close()
		tc := TileConfig{
			Width:   field.Image.Width,
			Height:  field.Image.Height,
			Spacing: field.Image.Spacing,
			UseMask: field.Image.UseMask,
		}
		tiles, computed, err := SplitImageToTiles(file, &tc)
		if err != nil {
			return 0, err
		}
		io.WriteString(header, MakeFxHeaderField("uint16_t", name+"Width", computed.SpriteWidth, 0))
		io.WriteString(header, MakeFxHeaderField("uint16_t", name+"Height", computed.SpriteHeight, 0))
		if len(tiles) > 1 {
			io.WriteString(header, MakeFxHeaderField("uint8_t", name+"Frames", len(tiles), 0))
		}
		for _, tile := range tiles {
			ptile, w, h := ImageToPaletted(tile, field.Image.Threshold, field.Image.AlphaThreshold)
			raw, mask, err := PalettedToRaw(ptile, w, h)
			if err != nil {
				return 0, err
			}
			truelength += len(raw)
			if field.Image.UseMask {
				truelength += len(mask)
			}
			for i := range raw {
				onebyte[0] = raw[i]
				data.Write(onebyte)
				if field.Image.UseMask {
					onebyte[0] = mask[i]
					data.Write(onebyte)
				}
			}
		}
		log.Printf("Copied image %s to fxdata (%d tiles)\n", field.Data, len(tiles))

	default:
		return 0, fmt.Errorf("Unknown format type %s", field.Format)
	}
	return truelength + position, nil
}

func MakeFxHeaderField(typ string, name string, value int, hex int) string {
	if hex > 0 {
		return fmt.Sprintf("constexpr %s %s = 0x%0*X;\n", typ, name, hex, value)
	} else {
		return fmt.Sprintf("constexpr %s %s = %d;\n", typ, name, value)
	}
}

// Return the line representing the full field at the given address.
// Only works for actual fxdata (don't use for FX_DATA_PAGE etc)
func MakeFxHeaderAddress(name string, addr int) string {
	return MakeFxHeaderField("uint24_t", name, addr, 6)
}

// Return the block representing a main fx pointer, such as FX_DATA_PAGE
// or FX_SAVE_PAGE
func MakeFxHeaderMainPointer(name string, addr uint, length uint) string {
	return fmt.Sprintf("%s%s\n",
		MakeFxHeaderField("uint16_t", name+"_PAGE", int(addr)/FXPageSize, 4),
		MakeFxHeaderField("uint24_t", name+"_BYTES", int(length), 0))
}

type FxOffsets struct {
	DataLength      int // real length of data as user defined it
	SaveLength      int // real length of save as user defined it
	DataLengthFlash int // length of data on flash (may be larger than DataLength)
	SaveLengthFlash int // length of save on flash (may be larger than SaveLength)
	DataStart       int // Beginning address (byte) of data
	SaveStart       int // Beginning address (byte) of save (will be past end of flash if no save)
}

// Parse the whole fx data and produce the header and all the little
// bits and pieces of binary data. To reduce memory usage, you must
// provide all the streams to the function for it to output data to.
// Returns the error and the length of data and save
// NOTE: THIS MUST BE USED ON A 16MB FLASH, due to how the FX libary
// works! I'm sorry!
func ParseFxData(data *FxData, header io.Writer, bin io.Writer) (*FxOffsets, error) {
	savepos := 0
	datapos := 0
	result := FxOffsets{}
	var err error

	io.WriteString(header, "#pragma once\n\nusing uint24_t = __uint24;\n\n")

	io.WriteString(header, "// Data fields (offsets into data section)\n")
	for key := range data.Data {
		datapos, err = ParseFxField(key, data.Data[key], header, bin, datapos)
		if err != nil {
			return nil, err
		}
	}

	// Gotta pad the data (if it exists...)
	result.DataLength = datapos
	result.DataLengthFlash = int(AlignWidth(uint(result.DataLength), uint(FXPageSize)))
	if result.DataLength > 0 {
		pad, err := bin.Write(MakePadding(result.DataLengthFlash - result.DataLength))
		if err != nil {
			return nil, err
		}
		log.Printf("Data padding is %d bytes\n", pad)
	}

	io.WriteString(header, "\n// Save fields (offsets into save section)\n")
	for key := range data.Save {
		savepos, err = ParseFxField(key, data.Save[key], header, bin, savepos)
		if err != nil {
			return nil, err
		}
	}

	// Gotta pad the save (if it exists...)
	result.SaveLength = savepos
	result.SaveLengthFlash = int(AlignWidth(uint(result.SaveLength), FxSaveAlignment))
	if result.SaveLength > 0 {
		pad, err := bin.Write(MakePadding(result.SaveLengthFlash - result.SaveLength))
		if err != nil {
			return nil, err
		}
		log.Printf("Save padding is %d bytes\n", pad)
	}

	// Figure out the positions
	result.SaveStart = FxDevExpectedFlashCapacity - result.SaveLengthFlash
	result.DataStart = result.SaveStart - result.DataLengthFlash

	// Write the positions (these usually go on top in the original fxdata.h, but
	// in ours, we write it at the bottom. Hopefully not much of a problem...)
	io.WriteString(header, "\n// FX addresses (only really used for initialization)\n")
	io.WriteString(header, MakeFxHeaderMainPointer("FX_DATA", uint(result.DataStart), uint(result.DataLength)))
	if result.SaveLength > 0 {
		io.WriteString(header, MakeFxHeaderMainPointer("FX_SAVE", uint(result.SaveStart), uint(result.SaveLength)))
	}

	io.WriteString(header, "// Helper macro to initialize fx, call in setup()\n")
	if result.SaveLength > 0 {
		io.WriteString(header, "#define FX_INIT() FX::begin(FX_DATA_PAGE, FX_DATA_SAVE)\n")
	} else {
		io.WriteString(header, "#define FX_INIT() FX::begin(FX_DATA_PAGE)\n")
	}

	return &result, nil
}
