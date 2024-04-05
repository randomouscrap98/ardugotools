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
	FxDevExpectedFlashCapacity = 2 ^ 24
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
	// if d.Type == "" {
	// 	d.Type = "uint8_t"
	// }
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

type FxDataOutput struct {
	Header io.Writer
	Data   io.ReadWriteSeeker
	Save   io.ReadWriteSeeker
	Dev    io.Writer
}

// Parse a single FX field, regardless of where it's supposed to go, and
// output the results to both the header and the data writer. Accepts the
// "current location" within the data writer, and should return the updated position
// within the data writer
func ParseFxField(name string, field *FxDataField, header *strings.Builder, data io.Writer,
	position int) (int, error) {
	truelength := 0
	onebyte := make([]byte, 1)
	// Preemptively write the header field now, and later we'll write some other junk
	header.WriteString(MakeFxHeaderAddress(name, position))
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
		header.WriteString(MakeFxHeaderField("uint16_t", name+"Width", computed.SpriteWidth, 0))
		header.WriteString(MakeFxHeaderField("uint16_t", name+"Height", computed.SpriteHeight, 0))
		if len(tiles) > 1 {
			header.WriteString(MakeFxHeaderField("uint8_t", name+"Frames", len(tiles), 0))
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
				onebyte[i] = raw[i]
				data.Write(onebyte)
				if field.Image.UseMask {
					onebyte[i] = mask[i]
					data.Write(onebyte)
				}
			}
		}
		log.Printf("Copied image %s to fxdata (%d tiles)\n", field.Data, len(tiles))

	default:
		return 0, fmt.Errorf("Unknown format type %s", field.Format)
	}
	// typ := field.Type
	// format := field.Format
	// if format == "" {
	//   format =
	// }
	// if typ == "" {
	//   typ = "uint8_t"
	// }
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
		MakeFxHeaderField("uint16_t", name+"_PAGE", int(addr), 4),
		MakeFxHeaderField("uint24_t", name+"_BYTES", int(addr), 0))
	// return fmt.Sprintf(
	// 	"constexpr uint16_t %s_PAGE = 0x%04X\nconstexpr uint24_t %s_BYTES = %d\n\n",
	// 	name, addr/uint(FXPageSize), name, length)
}

// Parse the whole fx data and produce the header and all the little
// bits and pieces of binary data. To reduce memory usage, you must
// provide all the streams to the function for it to output data to.
// Returns the error and the length of data and save
// NOTE: THIS MUST BE USED ON A 16MB FLASH, due to how the FX libary
// works! I'm sorry!
func ParseFxData(data *FxData, output *FxDataOutput) (int, int, error) {
	savepos := 0
	datapos := 0
	// Even though we don't really want the data to all reside in memory at
	// once, it's just easier if we put the header into a string builder.
	var sb strings.Builder
	var err error
	for key := range data.Data {
		datapos, err = ParseFxField(key, data.Data[key], &sb, output.Data, datapos)
		if err != nil {
			return 0, 0, err
		}
	}
	for key := range data.Save {
		savepos, err = ParseFxField(key, data.Data[key], &sb, output.Save, savepos)
		if err != nil {
			return 0, 0, err
		}
	}

	// Only at the end can we write everything to the header. Here, we know
	// the fx data and save size, and can output the proper thingies
	io.WriteString(output.Header, "#pragma once\n\nusing uint24_t = __uint24;\n\n")

	savelength := uint(savepos)
	datalength := uint(savepos)
	savelengthFlash := AlignWidth(savelength, FxSaveAlignment)
	datalengthFlash := AlignWidth(datalength, uint(FXPageSize))
	saveStart := FxDevExpectedFlashCapacity - savelengthFlash
	dataStart := saveStart - datalengthFlash

	// Write the padding; the files need to be pre-padded
	pad, err := output.Data.Write(MakePadding(int(datalengthFlash - datalength)))
	if err != nil {
		return 0, 0, err
	}
	log.Printf("Data padding is %d bytes\n", pad)
	if savelength > 0 {
		pad, err := output.Save.Write(MakePadding(int(savelengthFlash - savelength)))
		if err != nil {
			return 0, 0, err
		}
		log.Printf("Save padding is %d bytes\n", pad)
	}

	// Dump the data into the dev data; alignment is already there
	output.Data.Seek(0, io.SeekStart)
	_, err = io.Copy(output.Dev, output.Data)
	if err != nil {
		return 0, 0, err
	}

	// Can always write the fx data stuff
	io.WriteString(output.Header, MakeFxHeaderMainPointer("FX_DATA", dataStart, datalength))

	// Apparently can't always write the save (though it really should be safe...)
	if savelength > 0 {
		// Dump save into the dev data; alignment is already there
		output.Save.Seek(0, io.SeekStart)
		_, err = io.Copy(output.Dev, output.Save)
		if err != nil {
			return 0, 0, err
		}
		io.WriteString(output.Header, MakeFxHeaderMainPointer("FX_SAVE", saveStart, savelength))
	}

	// Finally, put the header into the actual place rather than in-memory
	io.WriteString(output.Header, sb.String())

	return int(datalength), int(savelength), nil
}
