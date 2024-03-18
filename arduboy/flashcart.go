package arduboy

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
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

// Read the entire flashcart slot-by-slot and write it out to the 'output' writer.
// This is a "smart" reader that scans through slots reading them one by one.
// This is slightly slower than mindless block reading, but can be overall faster
// because it's not reading the entire flash memory (plus you can get more
// interesting logging + data)
func ReadWholeFlashcart(sercon io.ReadWriter, output io.Writer, logProgress bool) (int, int, error) {
	headerAddr := 0
	headerCount := 0
	headerRaw := make([]byte, FxHeaderLength)
	maxBuffer := make([]byte, (1<<16)-FXPageSize) // MUST BE PAGE ALIGNED
	defer ResetRgbButtonState(sercon)

	for {
		// This should make a fun rainbow... well maybe it'll be fun...
		var rgbState uint8 = LEDCtrlBtnOff | uint8(headerCount&0b111)
		SetRgbButtonState(sercon, rgbState)

		// Read the header
		err := ReadFlashcartInto(sercon, uint16(headerAddr/FXPageSize), headerRaw)
		if err != nil {
			return 0, 0, err
		}

		// Parse the header. It might throw an "acceptable" error.
		header, _, err := ParseHeader(headerRaw)
		if err != nil {
			switch err.(type) {
			case *NotHeaderError: // This is fine, we're just at the end
				return headerAddr, headerCount, nil
			default:
				return 0, 0, err
			}
		}

		if logProgress {
			log.Printf("[%d] Reading: %s (%s - %s)\n", headerCount+1, header.Title, header.Developer, header.Version)
		}

		// Write the header. We'll be reading the rest of the slot now
		_, err = output.Write(headerRaw)
		if err != nil {
			return 0, 0, err
		}

		// The rest of the slot needs to be read. If for some reason this value
		// is 0, it will still work with the next loop
		nextSlot := headerAddr + int(header.SlotPages)*FXPageSize
		headerAddr += FxHeaderLength

		// Need to do a loop of reading as much as possible for this slot
		for headerAddr < nextSlot {
			// read in chunks of maxBuffer length. For sketches, we won't
			// ever reach the maxBuffer length. Also, the address should
			// move properly in page chunk sizes, since it's either (a) the
			// size of the slot (always page aligned) or (b) the maxBuffer
			// length (which is also page aligned)
			readBuf := maxBuffer[:min(nextSlot-headerAddr, len(maxBuffer))]
			err = ReadFlashcartInto(sercon, uint16(headerAddr/FXPageSize), readBuf)
			if err != nil {
				return 0, 0, err
			}
			_, err = output.Write(readBuf)
			if err != nil {
				return 0, 0, err
			}
			headerAddr += len(readBuf)
		}

		// Move to the next header
		headerCount++
	}
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
		// Fancy led strobing
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

		// Parse the header. It might throw an "acceptable" error.
		header, _, err := ParseHeader(headerRaw[:])
		if err != nil {
			switch err.(type) {
			case *NotHeaderError: // This is fine, we're just at the end
				return headerAddr, headerCount, nil
			default:
				return 0, 0, err
			}
		}

		// Call the user's function with the current state as we know it
		err = headerFunc(sercon, header, headerAddr, headerCount)
		if err != nil {
			return 0, 0, err
		}

		// Move to the next header
		headerCount++
		headerAddr += int(header.SlotPages) * FXPageSize
	}
}

// Same as ScanFlashcart but for a file / other file-like readerseeker. No address
// given, as that can be determined from the ReadSeeker given
func ScanFlashcartFile(data io.ReadSeeker, headerFunc func(io.ReadSeeker, *FxHeader, int) error) (int, error) {
	headerCount := 0
	headerRaw := make([]byte, FxHeaderLength)

ScanFlashcartLoop:
	for {
		_, err := io.ReadFull(data, headerRaw)
		if err != nil {
			if err == io.ErrUnexpectedEOF || err == io.EOF {
				break
			} else {
				return 0, err
			}
		}

		// Parse the header. It might throw an "acceptable" error.
		header, _, err := ParseHeader(headerRaw)
		if err != nil {
			switch err.(type) {
			case *NotHeaderError: // This is fine, we're just at the end
				break ScanFlashcartLoop
			default:
				return 0, err
			}
		}

		// Call the user's function with the current state as we know it
		err = headerFunc(data, header, headerCount)
		if err != nil {
			return 0, err
		}

		// Move to the next header
		headerCount++
		data.Seek(int64(header.NextPage)*int64(FXPageSize), io.SeekStart)
	}

	return headerCount, nil
}

type HeaderProgram struct {
	Title     string
	Version   string
	Developer string
	Info      string
	Sha256    string
	TotalSize int
	Image     string
}

type HeaderCategory struct {
	Title string
	Info  string
	Image string
	Slots []*HeaderProgram
}

// Given a header, store it in the appropriate place within the 'result'
// category list. This is a common operation for flashcart metadata scanning.
// Gives you the location where you can store the image (since both have the
// generic item)
func MapHeaderResult(result *[]HeaderCategory, header *FxHeader) (*string, error) {
	if header.IsCategory() {
		*result = append(*result, HeaderCategory{
			Title: header.Title,
			Info:  header.Info,
			Slots: make([]*HeaderProgram, 0),
		})
		return &(*result)[len(*result)-1].Image, nil
	} else {
		last := len(*result) - 1
		if last < 0 {
			return nil, errors.New("Invalid flashcart: did not start with a category!")
		}
		newProgram := &HeaderProgram{
			Title:     header.Title,
			Version:   header.Version,
			Developer: header.Developer,
			Info:      header.Info,
			Sha256:    header.Sha256,
			TotalSize: int(header.SlotPages) * FXPageSize,
		}
		(*result)[last].Slots = append((*result)[last].Slots, newProgram)
		return &newProgram.Image, nil
	}
}

// Scrape just the metadata out of the flashcart. Optionally pull images (much slower)
func ScanFlashcartMeta(sercon io.ReadWriter, getImages bool) ([]HeaderCategory, error) {
	result := make([]HeaderCategory, 0)
	errchan := make(chan error)

	scanFunc := func(con io.ReadWriter, header *FxHeader, addr int, headers int) error {
		// Dump the errors and quit the reader as soon as possible
		select {
		case err := <-errchan:
			if err != nil {
				return err
			}
		default:
		}
		// Where to eventually write the completed image (goroutine)
		writeimg, err := MapHeaderResult(&result, header)
		if err != nil {
			return err
		}
		// Pull images. The first part MUST be done synchronously, but the image conversion
		// can be run at any time
		if getImages {
			imgbytes, err := ReadFlashcart(con, uint16(addr/FXPageSize+1), uint16(ScreenBytes))
			if err != nil {
				return err
			}
			go func() {
				outbytes := RawToGrayscale(imgbytes, 0, 255)
				pngraw, err := GrayscaleToPng(outbytes)
				if err != nil {
					errchan <- err
				} else {
					*writeimg = "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngraw)
					errchan <- nil
				}
			}()
		}
		return nil
	}

	// Reading images is much slower, so the flash rate should speed up to match
	flashRate := 64
	if getImages {
		flashRate = 16
	}

	// Do the full flashcart scan, BUT the images might not be finished converting!
	_, _, err := ScanFlashcart(sercon, scanFunc, flashRate, LEDCtrlBlOn|LEDCtrlRdOn)
	if err != nil {
		return nil, err
	}

	// No error? Dump the errchan to be sure
ErrorDump:
	for {
		select {
		case err = <-errchan:
			if err != nil {
				return nil, err
			}
		default:
			break ErrorDump
		}
	}

	return result, nil
}

// Scrape metadata out of a file flashcart, same as ScanFlashcartMeta
func ScanFlashcartFileMeta(data io.ReadSeeker, getImages bool) ([]HeaderCategory, error) {
	result := make([]HeaderCategory, 0)
	imageRaw := make([]byte, ScreenBytes)
	data.Seek(0, io.SeekStart)

	scanFunc := func(con io.ReadSeeker, header *FxHeader, headerCount int) error {
		writeimg, err := MapHeaderResult(&result, header)
		if err != nil {
			return err
		}
		if getImages {
			_, err := io.ReadFull(con, imageRaw)
			if err != nil {
				return err
			}
			outbytes := RawToGrayscale(imageRaw, 0, 255)
			pngraw, err := GrayscaleToPng(outbytes)
			if err != nil {
				return err
			} else {
				*writeimg = "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngraw)
			}
		}
		return nil
	}

	_, err := ScanFlashcartFile(data, scanFunc)
	if err != nil {
		return nil, err
	}

	return result, nil
}
