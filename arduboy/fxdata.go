package arduboy

// NOTE: this file should not care about the format of the data or where it
// comes from, it should only be given the already-parsed data. As such,
// DON'T include things like the toml or json libararies!
import (
	"fmt"
	"io"
	"strings"
)

const (
	FxDevExpectedFlashCapacity = 2 ^ 24
)

// Information pertaining to how to parse the raw data from
// the fx data section.
// type FxDataParse struct {
// }

// A single field put into the fx data blob. May generate multiple
// fields based on configuration
type FxDataField struct {
	Type      string
	Import    string
	Raw       string
	Delimeter []byte
	Format    string
}

// All fields requested in the entire fx data blob. It must always
// be explicit
type FxData struct {
	Data []*FxDataField
	Save []*FxDataField
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
func ParseFxField(field *FxDataField, header *strings.Builder, data io.Writer,
	position int) (error, int) {
	return nil, 0
}

// Return the line representing the full field at the given address.
// Only works for actual fxdata (don't use for FX_DATA_PAGE etc)
func MakeFxHeaderField(name string, addr int) string {
	return fmt.Sprintf("constexpr uint24_t %s = 0x%06X;\n", name, addr)
}

// Return the block representing a main fx pointer, such as FX_DATA_PAGE
// or FX_SAVE_PAGE
func MakeFxHeaderMainPointer(name string, addr uint, length uint) string {
	return fmt.Sprintf(
		"constexpr uint16_t %s_PAGE = 0x%04X\nconstexpr uint24_t %s_BYTES = %d\n\n",
		name, addr/uint(FXPageSize), name, length)
}

// Parse the whole fx data and produce the header and all the little
// bits and pieces of binary data. To reduce memory usage, you must
// provide all the streams to the function for it to output data to.
// Returns the error and the length of data and save
// NOTE: THIS MUST BE USED ON A 16MB FLASH, due to how the FX libary
// works! I'm sorry!
func ParseFxData(data *FxData, output *FxDataOutput) (error, int, int) {
	savepos := 0
	datapos := 0
	// Even though we don't really want the data to all reside in memory at
	// once, it's just easier if we put the header into a string builder.
	var sb strings.Builder
	//io.WriteString(output.Header, fmt.Sprintf("// Generated on %s using ardugotools"))
	var err error
	for _, field := range data.Data {
		err, datapos = ParseFxField(field, &sb, output.Data, datapos)
		if err != nil {
			return err, 0, 0
		}
	}
	for _, field := range data.Save {
		err, savepos = ParseFxField(field, &sb, output.Save, savepos)
		if err != nil {
			return err, 0, 0
		}
	}

	// Only at the end can we write everything to the header. Here, we know
	// the fx data and save size, and can output the proper thingies
	io.WriteString(output.Header, "#pragma once\n\nusing uint24_t = __uint24;\n\n")

	savelength := uint(savepos)
	datalength := uint(savepos)
	saveStart := FxDevExpectedFlashCapacity - AlignWidth(savelength, FxSaveAlignment)
	dataStart := saveStart - AlignWidth(datalength, uint(FXPageSize))

	// Can always write the fx data stuff
	io.WriteString(output.Header, MakeFxHeaderMainPointer("FX_DATA", dataStart, datalength))

	// Apparently can't always write the save (though it really should be safe...)
	if savelength > 0 {
		io.WriteString(output.Header, MakeFxHeaderMainPointer("FX_SAVE", saveStart, savelength))
	}

	io.WriteString(output.Header, sb.String())
	return nil, int(datalength), int(savelength)
}
