package arduboy

// NOTE: this file should not care about the format of the data or where it
// comes from, it should only be given the already-parsed data. As such,
// DON'T include things like the toml or json libararies!
import (
	//"encoding/base64"
	//"encoding/hex"
	"fmt"
	//"io"
	//"log"
	//"os"
	//"strings"
)

const (
	FxDevExpectedFlashCapacity = 1 << 24
)

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

// type FxDataImageConfig struct {
// 	Width          int   // Width of tile (0 means use all available width)
// 	Height         int   // Height of tile (0 means use all available height)
// 	Spacing        int   // Spacing between tiles (including on edges)
// 	UseMask        bool  // Whether to use transparency as a data mask
// 	Threshold      uint8 // The upper bound for black pixels
// 	AlphaThreshold uint8 // The upper bound for alpha threshold
// }
//
// func (i *FxDataImageConfig) ReasonableDefaults() {
// 	if i.AlphaThreshold == 0 {
// 		i.AlphaThreshold = 10
// 	}
// 	if i.Threshold == 0 {
// 		i.Threshold = 100
// 	}
// }
//
// // A single field put into the fx data blob. May generate multiple
// // fields based on configuration
// type FxDataField struct {
// 	Data   string
// 	Format string
// 	Image  *FxDataImageConfig
// 	// This "delimeter" stuff might be over-stepping, and I just
// 	// don't know how much I want the data parser to do. It would be
// 	// nice not to require an external script to generate specially
// 	// formatted data, but there's a lot of ways people might want to
// 	// index their data. I think I will wait until a clear and repeated
// 	// need arises, then I will design it based on that need.
// 	//Delimeter []byte
// }
//
// func (d *FxDataField) ReasonableDefaults() {
// 	if d.Format == "" {
// 		d.Format = "file"
// 	}
// 	if d.Image == nil {
// 		d.Image = &FxDataImageConfig{}
// 	}
// 	d.Image.ReasonableDefaults()
// }
//
// // All fields requested in the entire fx data blob. It must always
// // be explicit
// type FxData struct {
// 	Data          map[string]*FxDataField
// 	Save          map[string]*FxDataField
// 	KeyOrder      []string
// 	MinSaveLength int
// }
//
// // Parse a single FX field, regardless of where it's supposed to go, and
// // output the results to both the header and the data writer. Accepts the
// // "current location" within the data writer, and should return the updated position
// // within the data writer
// func ParseFxField(name string, field *FxDataField, header io.Writer, data io.Writer,
// 	position int) (int, error) {
// 	if field == nil {
// 		return 0, fmt.Errorf("passed null 'field' in ParseFxField: %s", name)
// 	}
// 	truelength := 0
// 	onebyte := make([]byte, 1)
// 	// Preemptively write the header field now, and later we'll write some other junk
// 	io.WriteString(header, MakeFxHeaderAddress(name, position))
// 	switch strings.ToLower(field.Format) {
// 	case "file":
// 		// File is always raw: copy it directly to the output
// 		file, err := os.Open(field.Data)
// 		if err != nil {
// 			return 0, err
// 		}
// 		defer file.Close()
// 		copylen, err := io.Copy(data, file)
// 		if err != nil {
// 			return 0, err
// 		}
// 		log.Printf("%s: Copied raw file %s to fxdata (%d bytes)\n", name, field.Data, copylen)
// 		truelength = int(copylen)
// 	case "image":
// 		// Image is a bit more special: it needs to have a lot done to it, but
// 		// is eventually written almost as-is... kinda..
// 		file, err := os.Open(field.Data)
// 		if err != nil {
// 			return 0, err
// 		}
// 		defer file.Close()
// 		tc := TileConfig{
// 			Width:   field.Image.Width,
// 			Height:  field.Image.Height,
// 			Spacing: field.Image.Spacing,
// 			UseMask: field.Image.UseMask,
// 		}
// 		tiles, computed, err := SplitImageToTiles(file, &tc)
// 		if err != nil {
// 			return 0, err
// 		}
// 		io.WriteString(header, MakeFxHeaderField("uint16_t", name+"Width", computed.SpriteWidth, 0))
// 		io.WriteString(header, MakeFxHeaderField("uint16_t", name+"Height", computed.SpriteHeight, 0))
// 		if len(tiles) > 1 {
// 			io.WriteString(header, MakeFxHeaderField("uint8_t", name+"Frames", len(tiles), 0))
// 		}
// 		// Need to write the width and height as 2 byte fields
// 		preamble := make([]byte, 4)
// 		Write2ByteValue(uint16(computed.SpriteWidth), preamble, 0)
// 		Write2ByteValue(uint16(computed.SpriteHeight), preamble, 2)
// 		_, err = data.Write(preamble)
// 		if err != nil {
// 			return 0, err
// 		}
// 		truelength += 4
// 		// Now write all the tiles
// 		for _, tile := range tiles {
// 			ptile, w, h := ImageToPaletted(tile, field.Image.Threshold, field.Image.AlphaThreshold)
// 			raw, mask, err := PalettedToRaw(ptile, w, h)
// 			if err != nil {
// 				return 0, err
// 			}
// 			truelength += len(raw)
// 			if field.Image.UseMask {
// 				truelength += len(mask)
// 			}
// 			for i := range raw {
// 				onebyte[0] = raw[i]
// 				data.Write(onebyte)
// 				if field.Image.UseMask {
// 					onebyte[0] = mask[i]
// 					data.Write(onebyte)
// 				}
// 			}
// 		}
// 		log.Printf("%s: Copied image %s to fxdata (%d tiles)\n", name, field.Data, len(tiles))
// 	case "hex":
// 		sreader := strings.NewReader(field.Data)
// 		decoder := hex.NewDecoder(sreader)
// 		num, err := io.Copy(data, decoder)
// 		if err != nil {
// 			return 0, err
// 		}
// 		truelength += int(num)
// 		log.Printf("%s: Copied raw hex to fxdata (%d bytes)\n", name, truelength)
// 	case "base64":
// 		sreader := strings.NewReader(field.Data)
// 		decoder := base64.NewDecoder(base64.StdEncoding, sreader)
// 		num, err := io.Copy(data, decoder)
// 		if err != nil {
// 			return 0, err
// 		}
// 		truelength += int(num)
// 		log.Printf("%s: Copied base64 to fxdata (%d bytes)\n", name, truelength)
// 	case "string":
// 		sreader := strings.NewReader(field.Data)
// 		// Copy literal values
// 		num, err := io.Copy(data, sreader)
// 		if err != nil {
// 			return 0, err
// 		}
// 		// Also need to write the null terminator
// 		onebyte[0] = 0
// 		_, err = data.Write(onebyte)
// 		if err != nil {
// 			return 0, err
// 		}
// 		truelength += int(num) + 1
// 		log.Printf("%s: Copied string to fxdata (%d bytes)\n", name, truelength)
// 	default:
// 		return 0, fmt.Errorf("Unknown format type %s", field.Format)
// 	}
// 	io.WriteString(header, MakeFxHeaderAddress(name+"Length", truelength))
// 	return truelength + position, nil
// }

//// Parse the whole fx data and produce the header and all the little
//// bits and pieces of binary data. To reduce memory usage, you must
//// provide all the streams to the function for it to output data to.
//// Returns the error and the length of data and save
//// NOTE: THIS MUST BE USED ON A 16MB FLASH, due to how the FX libary
//// works! I'm sorry!
//func ParseFxData(data *FxData, header io.Writer, bin io.Writer) (*FxOffsets, error) {
//
//	// Both data and save operate the same way, so just make a function which can do both.
//	// TODO: This may be too generic and you may have to split this out again
//	parseSet := func(d map[string]*FxDataField, alignment int, minlength int, name string) (int, int, error) {
//		datapos := 0
//		var err error
//
//		for _, key := range SortedKeys(d, data.KeyOrder) {
//			datapos, err = ParseFxField(key, d[key], header, bin, datapos)
//			if err != nil {
//				return 0, 0, err
//			}
//		}
//
//		// Gotta pad the data (if it exists...)
//		length := datapos
//		lengthflash := int(AlignWidth(uint(max(length, minlength)), uint(alignment)))
//		if lengthflash > 0 {
//			pad, err := bin.Write(MakePadding(lengthflash - length))
//			if err != nil {
//				return 0, 0, err
//			}
//			log.Printf("%s padding is %d bytes\n", name, pad)
//		}
//		return length, lengthflash, nil
//	}
//
//	result := FxOffsets{}
//	var err error
//
//	io.WriteString(header, "#pragma once\n\nusing uint24_t = __uint24;\n\n")
//
//	io.WriteString(header, "// Data fields (offsets into data section)\n")
//	result.DataLength, result.DataLengthFlash, err = parseSet(data.Data, FXPageSize, 0, "Data")
//	if err != nil {
//		return nil, err
//	}
//
//	io.WriteString(header, "\n// Save fields (offsets into save section)\n")
//	result.SaveLength, result.SaveLengthFlash, err = parseSet(data.Save, FxSaveAlignment, data.MinSaveLength, "Save")
//	if err != nil {
//		return nil, err
//	}
//
//	// Figure out the positions
//	result.SaveStart = FxDevExpectedFlashCapacity - result.SaveLengthFlash
//	result.DataStart = result.SaveStart - result.DataLengthFlash
//
//	// Write the positions (these usually go on top in the original fxdata.h, but
//	// in ours, we write it at the bottom. Hopefully not much of a problem...)
//	io.WriteString(header, "\n// FX addresses (only really used for initialization)\n")
//	io.WriteString(header, MakeFxHeaderMainPointer("FX_DATA", uint(result.DataStart), uint(result.DataLength)))
//	if max(result.SaveLengthFlash) > 0 {
//		io.WriteString(header, MakeFxHeaderMainPointer("FX_SAVE", uint(result.SaveStart), uint(result.SaveLength)))
//	}
//
//	io.WriteString(header, "// Helper macro to initialize fx, call in setup()\n")
//	if result.SaveLengthFlash > 0 {
//		io.WriteString(header, "#define FX_INIT() FX::begin(FX_DATA_PAGE, FX_DATA_SAVE)\n")
//	} else {
//		io.WriteString(header, "#define FX_INIT() FX::begin(FX_DATA_PAGE)\n")
//	}
//
//	return &result, nil
//}
