package arduboy

import (
	"bytes"
	"fmt"
)

const (
	FxHeaderStartString = "ARDUBOY"

	FxHeaderLength      = 256                                         // The flashcart slot header length in bytes
	FxHeaderMetaSize    = 199                                         // Length of the metadata section
	FxHeaderImageLength = 1024                                        // The flashcart slot title image length in bytes
	FxPreamblePages     = (FxHeaderLength + FxHeaderImageLength) >> 8 // Page size of entire preamble (includes title)
	FxSaveAlignment     = 4096                                        // Saves must be aligned to this size

	// MAX_PROGRAM_LENGTH = HEADER_PROGRAM_FACTOR * 0xFFFF
	// THis is for the programs that have a final page that's not required.
	// PROGRAM_NULLPAGE = b'\xFF' * HEADER_PROGRAM_FACTOR

	FxHeaderCategoryIndex     = 7  // "Index into slot header for" category (1 byte)
	FxHeaderPreviousPageIndex = 8  // "" previous slot page (2 bytes)
	FxHeaderNextPageIndex     = 10 // "" next slot page (2 bytes)
	FxHeaderSlotSizeIndex     = 12 // "" slot size. (2 bytes)
	FxHeaderProgramSizeIndex  = 14 // "" program size (1 byte, factor of 128)
	FxHeaderProgramPageIndex  = 15 // "" starting page of program (2 bytes)
	FxHeaderDataPageIndex     = 17 // "" starting page of data (2 bytes)
	FxHeaderSavePageIndex     = 19 // "" starting page of save (2 bytes)
	FxHeaderDataSizeIndex     = 21 // "" data segment size (2 bytes, factor of 256)
	FxHeaderMetaIndex         = 57 // "" metadata
)

// All data in the header of an fx slot (JUST the header, not the image)
type FxHeader struct {
	Category     uint8
	PreviousPage uint16
	NextPage     uint16
	SlotPages    uint16
	ProgramPages uint8
	ProgramStart uint16
	DataStart    uint16
	SaveStart    uint16
	DataPages    uint16

	// Metadata
	Title     string
	Version   string
	Developer string
	Info      string
}

func (h *FxHeader) IsCategory() bool {
	return h.ProgramStart == 0xFFFF
}

func FxHeaderStartBytes() []byte {
	return []byte(FxHeaderStartString)
}

// Generate the bytes you can write to the flashcart
func (header *FxHeader) MakeHeader() [FxHeaderLength]byte {
	var result [FxHeaderLength]byte

	return result
}

type NotEnoughDataError struct {
	Expected int
	Found    int
}

func (m *NotEnoughDataError) Error() string {
	return fmt.Sprintf("Not enough data: expected %d, got %d", m.Expected, m.Found)
}

type NotHeaderError struct{}

func (m *NotHeaderError) Error() string {
	return fmt.Sprintf("Data contains no header")
}

// Parse the header out of a byte slice. Will throw an error
// on slice too small or on header "not a header". Byte array returned
// is the slice without the header anymore
func ParseHeader(data []byte) (*FxHeader, []byte, error) {
	if len(data) < FxHeaderLength {
		return nil, nil, &NotEnoughDataError{Expected: FxHeaderLength, Found: len(data)}
	}
	headerBytes := FxHeaderStartBytes()
	if !bytes.HasPrefix(data, headerBytes) {
		return nil, nil, &NotHeaderError{}
	}
	result := FxHeader{
		Category:     data[FxHeaderCategoryIndex],
		PreviousPage: Get2ByteValue(data, FxHeaderPreviousPageIndex),
		NextPage:     Get2ByteValue(data, FxHeaderNextPageIndex),
		SlotPages:    Get2ByteValue(data, FxHeaderSlotSizeIndex),
		ProgramPages: data[FxHeaderProgramSizeIndex],
		ProgramStart: Get2ByteValue(data, FxHeaderProgramPageIndex),
		DataStart:    Get2ByteValue(data, FxHeaderDataPageIndex),
		SaveStart:    Get2ByteValue(data, FxHeaderSavePageIndex),
		DataPages:    Get2ByteValue(data, FxHeaderDataSizeIndex),
	}

	metaStrings := ParseStringArray(data[FxHeaderMetaIndex:FxHeaderLength])
	mlen := len(metaStrings)
	for i := mlen; i < 4; i++ {
		metaStrings = append(metaStrings, "")
	}

	if result.IsCategory() {
		// This is a category, it has special needs
		result.Title = metaStrings[0]
		result.Info = metaStrings[1]
	} else {
		// This is a regular slot, try to get all the fields
		result.Title = metaStrings[0]
		result.Version = metaStrings[1]
		result.Developer = metaStrings[2]
		result.Info = metaStrings[3]
	}

	return &result, data[FxHeaderLength:], nil
}
