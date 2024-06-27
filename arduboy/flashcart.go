package arduboy

import (
	"bytes"
	"crypto/sha256"
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

func (h *FxHeader) HasFxData() bool {
	return h.DataStart != 0xFFFF
}

func (h *FxHeader) HasFxSave() bool {
	return h.SaveStart != 0xFFFF
}

// We didn't always utilize that data pages field. If there's actual
// FX data but the data pages field is "unset", this is the old format.
func (h *FxHeader) IsOldFormat() bool {
	return h.HasFxData() && h.DataPages == 0xFFFF
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

func calculateHeaderHash(sketch []byte, fxdata []byte) (string, error) {
	both := make([]byte, len(sketch)+len(fxdata))
	copy(both, sketch)
	copy(both[len(sketch):], fxdata)
	hash := sha256.Sum256(both)
	return hex.EncodeToString(hash[:]), nil
	//hasher := sha256.N.New()
	//_, err := hasher.Write(sketch)
	//if err != nil {
	//	return "", err
	//}
	//_, err = hasher.Write(fxdata)
	//if err != nil {
	//	return "", err
	//}
	//return hex.EncodeToString(hasher.Sum256(nil)), nil
}

// Read any portion of flashcart at given address. Not performant at all
func ReadFlashcartInto(sercon io.ReadWriter, address int, length int, output io.Writer, readbuf []byte) error {
	page := uint16(address / FXPageSize)
	skip := address - int(page)*FXPageSize
	totalLength := length + skip
	if readbuf == nil {
		readbuf = CreateReadFlashcartBuffer()
	}
	readLength := (len(readbuf) / FXPageSize) * FXPageSize
	if readLength == 0 {
		return fmt.Errorf("buffer too small! min: %d", FXPageSize)
	}
	rwep := ReadWriteErrorPass{rw: sercon}
	sb := make([]byte, 1)

	for addressOffset := 0; addressOffset < totalLength; addressOffset += readLength {
		readAddress := address + addressOffset
		rwep.WritePass(AddressCommandFlashcartPage(uint16(readAddress / FXPageSize)))
		rwep.ReadPass(sb)
		readNow := min(totalLength-addressOffset, readLength)
		log.Printf("readAddress: %d, readNow: %d, skip: %d", readAddress, readNow, skip)
		rwep.WritePass(ReadFlashcartCommand(uint16(readNow)))
		rwep.ReadPass(readbuf[:readNow])
		output.Write(readbuf[skip:readNow])
		skip = 0
	}
	return rwep.err
}

// Create the optimal buffer needed for ReadFlashcartInto
func CreateReadFlashcartBuffer() []byte {
	return make([]byte, FXBlockSize)
}

// Read portion of flashcart at given address, allocating a new slice every time.
// Very much not performant; prefer ReadFlashcartOptimized if possible
func ReadFlashcart(sercon io.ReadWriter, address int, length int) ([]byte, error) {
	result := bytes.NewBuffer(make([]byte, 0, length))
	err := ReadFlashcartInto(sercon, address, length, result, nil)
	return result.Bytes(), err
}

// Read flashcart at simple page boundaries and in less-than-16bit lengths. Function
// will NOT throw an error if data is too long, it will simply only read up to 65k
func ReadFlashcartOptimizedInto(sercon io.ReadWriter, page uint16, data []byte) error {
	rwep := ReadWriteErrorPass{rw: sercon}
	sb := make([]byte, 1)
	rwep.WritePass(AddressCommandFlashcartPage(page))
	rwep.ReadPass(sb)
	readLength := min(len(data), FXBlockSize)
	rwep.WritePass(ReadFlashcartCommand(uint16(readLength)))
	rwep.ReadPass(data[:readLength])
	return rwep.err
}

// Read flashcart at simple page boundaries, creating a new slice every time
func ReadFlashcartOptimized(sercon io.ReadWriter, page uint16, length uint16) ([]byte, error) {
	result := make([]byte, length)
	err := ReadFlashcartOptimizedInto(sercon, page, result)
	return result, err
}

// Write any arbitrary amount of data to the flash, perserving any data surrounding
// it (since writing flashes the entire 65k block). Return the actual address it started
// writing to, and the total write size
func WriteFlashcart(sercon io.ReadWriter, address int, data []byte, logProgress bool) (int, int, error) {
	blockAlignedAddress := (address / FXBlockSize) * FXBlockSize
	backfillLength := address - blockAlignedAddress
	// Read backfill to get the data block aligned and not accidentally clear pre data
	if backfillLength > 0 {
		if logProgress {
			log.Printf("Reading backfill to preserve block data: %d bytes at address %d", backfillLength, blockAlignedAddress)
		}
		backfill, err := ReadFlashcart(sercon, blockAlignedAddress, backfillLength)
		if err != nil {
			return 0, 0, err
		}
		// This is apparently performant...
		data = append(backfill, data...)
	}
	overflow := len(data) % FXBlockSize
	if overflow > 0 {
		leftover := FXBlockSize - overflow
		readAddress := blockAlignedAddress + len(data)
		if logProgress {
			log.Printf("Reading %d bytes of leftover data at address %d", leftover, readAddress)
		}
		leftoverData, err := ReadFlashcart(sercon, readAddress, leftover)
		if err != nil {
			return 0, 0, err
		}
		data = append(data, leftoverData...)
	}

	if len(data)%FXBlockSize > 0 {
		return 0, 0, fmt.Errorf("PROGRAM ERROR: constructed fx data not block sized (%d): %d", FXBlockSize, len(data))
	}

	blocknum := 0
	rwep := ReadWriteErrorPass{rw: sercon}
	onebyte := make([]byte, 1)

	//defer ResetRgbButtonState(sercon)
	for i := 0; i < len(data); i += FXBlockSize {
		//var rgbState uint8 = LEDCtrlBtnOff | uint8(i&0b111)
		//SetRgbButtonState(sercon, rgbState)
		if logProgress {
			log.Printf("Writing block# %d at page %d", blocknum, i)
		}
		rwep.WritePass(AddressCommandFlashcartPage(uint16((blockAlignedAddress + i) / FXPageSize)))
		rwep.ReadPass(onebyte)
		rwep.WritePass(WriteFlashcartCommand(0)) //Yes, apparently it's 0 for full block
		rwep.WritePass(data[i : i+FXBlockSize])
		rwep.ReadPass(onebyte)
		if rwep.err != nil {
			return 0, 0, rwep.err
		}
		blocknum++
	}

	return blockAlignedAddress, len(data), nil
}

// Read the entire flashcart slot-by-slot and write it out to the 'output' writer.
// This is a "smart" reader that scans through slots reading them one by one.
// This is slightly slower than mindless block reading, but can be overall faster
// because it's not reading the entire flash memory (plus you can get more
// interesting logging + data). NOTE: DOES NOT CHECK FOR FLASHCART EXISTENCE!
func ReadWholeFlashcart(sercon io.ReadWriter, output io.Writer, logProgress bool) (int, int, error) {
	headerAddr := 0
	headerCount := 0
	headerRaw := make([]byte, FxHeaderLength)
	readBuffer := CreateReadFlashcartBuffer()
	defer ResetRgbButtonState(sercon)

	for {
		// This should make a fun rainbow... well maybe it'll be fun...
		var rgbState uint8 = LEDCtrlBtnOff | uint8(headerCount&0b111)
		SetRgbButtonState(sercon, rgbState)

		// Read the header
		err := ReadFlashcartOptimizedInto(sercon, uint16(headerAddr/FXPageSize), headerRaw)
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

		slotSize := int(header.SlotPages) * FXPageSize

		if logProgress {
			log.Printf("[%d] Reading: %s (%s - %s) - %d bytes\n",
				headerCount+1, header.Title, header.Developer, header.Version, slotSize)
		}

		// Write the header. We'll be reading the rest of the slot now
		_, err = output.Write(headerRaw)
		if err != nil {
			return 0, 0, err
		}

		// Read the rest of the slot (skipping the header)
		err = ReadFlashcartInto(sercon, headerAddr+FxHeaderLength, slotSize-FxHeaderLength, output, readBuffer)
		if err != nil {
			return 0, 0, err
		}

		// Move to the next header
		headerAddr += slotSize
		headerCount++
	}
}

// Write an entire flashcart starting at the normal address and going to the end.
// Does not care about any existing data on the flashcart.
// NOTE: DOES NOT CHECK FOR FLASHCART EXISTENCE OR SIZE
func WriteWholeFlashcart(sercon io.ReadWriter, input io.Reader, verify bool, logProgress bool) (int, error) {
	currentBlock := 0
	// This writer writes FULL fx blocks (its smallest writable chunk size). This is
	// beneficial: we will fill unused bytes with 0xFF (there should only be one
	// instance), and this combined with the multireader:
	// - makes sure a full page of 0xFF is written at the end
	// - guarantees the flashcart is "page aligned" even if it's not, because
	//   technically we're aligning it to the whole dang block
	bufferRaw := make([]byte, FXBlockSize)
	compareBuffer := make([]byte, FXBlockSize)
	onebyte := make([]byte, 1)
	endPage := make([]byte, FXPageSize)
	for i := range endPage {
		endPage[i] = 0xFF
	}
	endReader := bytes.NewReader(endPage)
	flashcartReader := io.MultiReader(input, endReader)
	rwep := ReadWriteErrorPass{rw: sercon}
	defer ResetRgbButtonState(sercon)

	running := true

	for running {
		// This should make a fun rainbow... well maybe it'll be fun...
		var rgbState uint8 = LEDCtrlBtnOff | uint8(currentBlock&0b111)
		if currentBlock&0b111 == 0 {
			// Don't let it be dark ever
			rgbState |= LEDCtrlRdOn
		}
		SetRgbButtonState(sercon, rgbState)

		// Read data from the input
		actual, err := io.ReadFull(flashcartReader, bufferRaw)

		if err != nil {
			if err == io.EOF {
				// You somehow hit the end of file right on the mark. Nice? IDK,
				// just quit now, nothing else to do (really)
				break
			} else if err == io.ErrUnexpectedEOF {
				// This is the usual end of flashcart. We didn't quite reach a full
				// blocksize, so fill the rest of the buffer; this will be the last iteration
				for i := actual; i < len(bufferRaw); i++ {
					bufferRaw[i] = 0xFF
				}
				running = false
			} else {
				// Wow, some other error! Fancy... but also we die
				return 0, err
			}
		}

		if logProgress {
			log.Printf("Writing block %d (%d bytes written)\n", currentBlock, currentBlock*FXBlockSize)
		}

		// everything uses flashcart pages so....
		currentPage := uint16(currentBlock * FxPagesPerBlock)

		// Write the data to the device
		rwep.WritePass(AddressCommandFlashcartPage(currentPage))
		rwep.ReadPass(onebyte)
		rwep.WritePass(WriteFlashcartCommand(0)) //Yes, apparently it's 0
		rwep.WritePass(bufferRaw)
		rwep.ReadPass(onebyte)

		if rwep.err != nil {
			return 0, rwep.err
		}

		// Now verify the data
		if verify {
			// Turn off LEDs for... I don't know, SOME kind of indication?
			SetRgbButtonState(sercon, LEDCtrlBtnOff)
			err = ReadFlashcartOptimizedInto(sercon, currentPage, compareBuffer)
			if err != nil {
				return 0, err
			}
			if !bytes.Equal(bufferRaw, compareBuffer) {
				return 0, fmt.Errorf("Flashcart validation failed at block %d!", currentBlock)
			}
		}

		// Move to the next block
		currentBlock++
	}

	return currentBlock, nil
}

// Scan through the flashcart, calling the given function for each header
// parsed. returns the total size of the flashcart and the number of
// headers read. The function also receives the current header address and
// number of headers previously read (starts with 0).
// NOTE: DOES NOT CHECK FOR FLASHCART EXISTENCE!!!
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
		err := ReadFlashcartOptimizedInto(sercon, uint16(headerAddr/FXPageSize), headerRaw[:])
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

// Same as ScanFlashcart but for a file / other file-like readerseeker.
func ScanFlashcartFile(data io.ReadSeeker, headerFunc func(io.ReadSeeker, *FxHeader, int, int) error) (int, error) {
	headerCount := 0
	headerRaw := make([]byte, FxHeaderLength)

ScanFlashcartLoop:
	for {
		startAddress, err := data.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		_, err = io.ReadFull(data, headerRaw)
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
		err = headerFunc(data, header, int(startAddress), headerCount)
		if err != nil {
			return 0, err
		}

		// Move to the next header
		headerCount++
		_, err = data.Seek(int64(header.NextPage)*int64(FXPageSize), io.SeekStart)
		if err != nil {
			return 0, err
		}
	}

	return headerCount, nil
}

// A wrapper for ScanFlashcart which only returns the basic flashcart size in bytes and slots
func ScanFlashcartSize(sercon io.ReadWriter) (int, int, error) {
	scanFunc := func(con io.ReadWriter, header *FxHeader, addr int, headers int) error {
		return nil
	}
	return ScanFlashcart(sercon, scanFunc, 64, LEDCtrlBlOn|LEDCtrlRdOn)
}

type HeaderProgram struct {
	Title     string
	Version   string
	Developer string
	Info      string
	Sha256    string
	SlotSize  int
	Address   int
	Image     string
	// NOTE: CAN'T DO THESE: the sizes indicated in the header do NOT
	// give information about how to read FROM the header!
	//SketchSize int
	//FxDataSize int
	//FxSaveSize int
}

type HeaderCategory struct {
	Title    string
	Info     string
	Image    string
	Address  int
	SlotSize int
	Slots    []*HeaderProgram
}

// Given a header, store it in the appropriate place within the 'result'
// category list. This is a common operation for flashcart metadata scanning.
// Gives you the location where you can store the image (since both have the
// generic item)
func MapHeaderResult(result *[]HeaderCategory, header *FxHeader, addr int) (*string, error) {
	if header.IsCategory() {
		*result = append(*result, HeaderCategory{
			Title:    header.Title,
			Info:     header.Info,
			Address:  addr,
			SlotSize: int(header.SlotPages) * FXPageSize,
			Slots:    make([]*HeaderProgram, 0),
		})
		return &(*result)[len(*result)-1].Image, nil
	} else {
		last := len(*result) - 1
		if last < 0 {
			return nil, errors.New("invalid flashcart: did not start with a category")
		}

		newProgram := &HeaderProgram{
			Title:     header.Title,
			Version:   header.Version,
			Developer: header.Developer,
			Info:      header.Info,
			Sha256:    header.Sha256,
			Address:   addr,
			SlotSize:  int(header.SlotPages) * FXPageSize,
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
		writeimg, err := MapHeaderResult(&result, header, addr)
		if err != nil {
			return err
		}
		// Pull images. The first part MUST be done synchronously, but the image conversion
		// can be run at any time
		if getImages {
			imgbytes, err := ReadFlashcartOptimized(con, uint16(addr/FXPageSize+1), uint16(ScreenBytes))
			if err != nil {
				return err
			}
			go func() {
				outbytes, err := RawToPalettedTitle(imgbytes)
				if err != nil {
					errchan <- err
					return
				}
				pngraw, err := PalettedToImageTitleBW(outbytes, "png")
				if err != nil {
					errchan <- err
					return
				}
				*writeimg = "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngraw)
				errchan <- nil
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

	scanFunc := func(con io.ReadSeeker, header *FxHeader, addr int, headerCount int) error {
		writeimg, err := MapHeaderResult(&result, header, addr)
		if err != nil {
			return err
		}
		if getImages {
			_, err := io.ReadFull(con, imageRaw)
			if err != nil {
				return err
			}
			outbytes, err := RawToPalettedTitle(imageRaw)
			if err != nil {
				return err
			}
			pngraw, err := PalettedToImageTitleBW(outbytes, "png")
			if err != nil {
				return err
			}
			*writeimg = "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngraw)
		}
		return nil
	}

	_, err := ScanFlashcartFile(data, scanFunc)
	if err != nil {
		return nil, err
	}

	return result, nil
}
