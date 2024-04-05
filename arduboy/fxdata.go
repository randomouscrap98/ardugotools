package arduboy

// NOTE: this file should not care about the format of the data or where it
// comes from, it should only be given the already-parsed data. As such,
// DON'T include things like the toml or json libararies!
import (
	"fmt"
	"io"
	"log"
	"strings"
)

const (
	FxDevExpectedFlashCapacity = 2 ^ 24
)

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
	position int) (int, error) {
	return 0, nil
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
func ParseFxData(data *FxData, output *FxDataOutput) (int, int, error) {
	savepos := 0
	datapos := 0
	// Even though we don't really want the data to all reside in memory at
	// once, it's just easier if we put the header into a string builder.
	var sb strings.Builder
	var err error
	for _, field := range data.Data {
		datapos, err = ParseFxField(field, &sb, output.Data, datapos)
		if err != nil {
			return 0, 0, err
		}
	}
	for _, field := range data.Save {
		savepos, err = ParseFxField(field, &sb, output.Save, savepos)
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
