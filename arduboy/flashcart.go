package arduboy

import ()

const (
	FxHeaderStartString = "ARDUBOY"
	// HeaderStartBytes = bytearray(HEADER_START_STRING.encode())

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
	DataPages    uint16

	// Metadata
	Title     string
	Version   string
	Developer string
	Info      string
}
