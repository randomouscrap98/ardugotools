package arduboy

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"log"
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
	FxHeaderHashIndex         = 25 // "" hash (32 bytes)
	FxHeaderHashLength        = 32
	FxHeaderMetaIndex         = 57 // "" metadata
)

func FxHeaderStartBytes() []byte {
	return []byte(FxHeaderStartString)
}

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

	Sha256 string

	// Metadata
	Title     string
	Version   string
	Developer string
	Info      string
}

func (h *FxHeader) IsCategory() bool {
	return h.ProgramStart == 0xFFFF
}

// Generate the bytes you can write to the flashcart
func (header *FxHeader) MakeHeader() ([]byte, error) {
	result := make([]byte, FxHeaderLength)
	for i := range result {
		result[i] = 0xFF
	}

	// Copy the header start bytes
	copy(result[:], FxHeaderStartBytes())

	// Write the normal-ish values
	result[FxHeaderCategoryIndex] = header.Category
	Write2ByteValue(header.PreviousPage, result, FxHeaderPreviousPageIndex)
	Write2ByteValue(header.NextPage, result, FxHeaderNextPageIndex)
	Write2ByteValue(header.SlotPages, result, FxHeaderSlotSizeIndex)
	result[FxHeaderProgramSizeIndex] = header.ProgramPages
	Write2ByteValue(header.ProgramStart, result, FxHeaderProgramPageIndex)
	Write2ByteValue(header.DataStart, result, FxHeaderDataPageIndex)
	Write2ByteValue(header.SaveStart, result, FxHeaderSavePageIndex)
	Write2ByteValue(header.DataPages, result, FxHeaderDataSizeIndex)

	// Write the ugly hash (it's a hex string)
	hash, err := hex.DecodeString(header.Sha256)
	if err != nil {
		return nil, err
	}
	copy(result[FxHeaderHashIndex:FxHeaderHashIndex+FxHeaderHashLength], hash)

	// And now the metadata
	metastrings := make([]string, 0)

	if header.IsCategory() {
		metastrings = append(metastrings, header.Title)
		metastrings = append(metastrings, header.Info)
	} else {
		metastrings = append(metastrings, header.Title)
		metastrings = append(metastrings, header.Version)
		metastrings = append(metastrings, header.Developer)
		metastrings = append(metastrings, header.Info)
	}

	// Write the stupid metadata
	stop, trunc := FillStringArray(metastrings, result[FxHeaderMetaIndex:FxHeaderMetaIndex+FxHeaderMetaSize])
	if stop != len(metastrings) || trunc != 0 {
		log.Printf("Couldn't write all metastrings: stopped on string [%d], truncated %d", stop, trunc)
	}

	return result, nil
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
		Sha256:       hex.EncodeToString(data[FxHeaderHashIndex : FxHeaderHashIndex+FxHeaderHashLength]),
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

// Read portion of flashcart at given page
func ReadFlashcartInto(sercon io.ReadWriter, page uint16, data []byte) error {
	rwep := ReadWriteErrorPass{rw: sercon}
	rwep.WritePass(AddressCommandFlashcartPage(page))
	var sb [1]byte
	rwep.ReadPass(sb[:])
	rwep.WritePass(ReadFlashcartCommand(uint16(len(data))))
	rwep.ReadPass(data)
	return rwep.err
}

// Read portion of flashcart at given page, allocating a new slice every time
func ReadFlashcart(sercon io.ReadWriter, page uint16, length uint16) ([]byte, error) {
	result := make([]byte, length)
	err := ReadFlashcartInto(sercon, page, result)
	return result, err
}

// Scan through the flashcart, calling the given function for each header
// parsed. returns the total size of the flashcart and the number of
// headers read. The function also receives the current header address and
// number of headers previously read (starts with 0)
func ScanFlashcart(sercon io.ReadWriter, headerFunc func(io.ReadWriter, *FxHeader, int, int) error,
	flashRate int, flashColor uint8) (int, int, error) {
	headerAddr := 0
	headerCount := 0
	var headerRaw [FxHeaderLength]byte
	var lastState uint8 = 0
	var thisState uint8 = LEDCtrlBtnOff
	defer ResetRgbButtonState(sercon)

	for {
		if flashRate > 0 {
			thisState = LEDCtrlBtnOff | (flashColor * byte((headerCount/flashRate)&1))
		}
		if thisState != lastState {
			SetRgbButtonState(sercon, thisState)
			lastState = thisState
		}

		// Now for the ACTUAL reading
		err := ReadFlashcartInto(sercon, uint16(headerAddr/FXPageSize), headerRaw[:])
		if err != nil {
			return 0, 0, err
		}
		header, _, err := ParseHeader(headerRaw[:])
		if err != nil {
			switch err.(type) {
			case *NotHeaderError: // This is fine, we're just at the end
				return headerAddr, headerCount, nil
			default:
				return 0, 0, err
			}
		}
		err = headerFunc(sercon, header, headerAddr, headerCount)
		if err != nil {
			return 0, 0, err
		}
		headerCount++
		headerAddr += int(header.SlotPages) * FXPageSize
	}
}

// def scan_fx(s_port, header_work = None, report_progress = None):
//     """
//     Scan through the device's FX flash memory, calling the given function for
//     every read header. Continues until header_work returns false, or the end
//     of the cart is reached. The size of the flashcart is returned.
//     """
//
//     ## detect flash cart ##
//     jedec_info = get_and_verify_jdec_bootloader(s_port)
//
//     header_addr = 0     # The actual current byte address
//     slots = 0           # The number of slots
//
//     # We don't know when we'll reach the end of the cart, we have to parse the headers
//     while True:
//         if (slots // 64) & 1:
//             s_port.write(b"x\xC0") #RGB BLUE OFF, buttons disabled
//         else:
//             s_port.write(b"x\xC1") #RGB BLUE RED, buttons disabled
//         s_port.read(1)
//
//         # Read just the header bytes
//         blockaddr = header_addr // FX_PAGESIZE
//         readlen = arduboy.fxcart.HEADER_LENGTH
//         s_port.write(bytearray([ord("A"), blockaddr >> 8, blockaddr & 0xFF]))
//         s_port.read(1)
//         s_port.write(bytearray([ord("g"), (readlen >> 8) & 0xFF, readlen & 0xFF, ord("C")]))
//
//         header = s_port.read(readlen)
//
//         # This chunk of flash indicates the end of the cart, because it was not a header!
//         if not arduboy.fxcart.is_slot(header, 0):
//             break
//
//         slots += 1
//
//         # Or if the user tells us to stop, we do.
//         if header_work:
//             if not header_work(header, header_addr):
//                 break
//
//         # Move to the next apparent slot
//         header_addr += arduboy.fxcart.get_slot_size_bytes(header, 0)
//
//         if report_progress:
//             report_progress(header_addr, jedec_info.capacity)
//
//     s_port.write(b"x\x40")#RGB LED off, buttons enabled
//     s_port.read(1)
//
//     return header_addr, slots
