package arduboy

import (
	"bytes"
	"fmt"
	"io"
	"log"

	"github.com/marcinbor85/gohex"
)

const (
	HexLineLength = 16
)

type SketchAnalysis struct {
	OverwritesCaterina bool
	OverwritesCathy    bool // If this happens, sketch is too large. Probably not used...
	TotalPages         int
	TrimmedData        []byte
	DetectedDevice     string
}

var (
	ARDUBOYFXEnableBytes    = []byte{0x59, 0x98}
	ARDUBOYFXDisableBytes   = []byte{0x59, 0x9a}
	ARDUBOYMINIEnableBytes  = []byte{0x72, 0x98} //, 0x0e, 0x94}
	ARDUBOYMINIDisableBytes = []byte{0x72, 0x9a} //, 0x08, 0x95}
	ARDUBOYCallFollowBytes  = [][]byte{
		{0x08, 0x95},
		{0x0e, 0x94},
		{0x83, 0xe0, 0x0e, 0x94}, // Manic miner had this instead
	}
)

// Simply find the given instruction sequence within the given sketch binary. ensures
// 16 bit alignment (instructions are 16 bit)
func findInstructionSequence(bindata []byte, sequence []byte) bool {
	pos := bytes.Index(bindata, sequence)
	return pos >= 0 && (pos&1) == 0
}

// Search for any form of "call return" (there are many types generated, we may not
// have them all stored in ARDUBOYCallFollowBytes) AFTER the given initial bytes. So,
// that would be some sequence of instructions followed by a call/return
func findCallRet(bindata []byte, initial []byte) bool {
	for _, fb := range ARDUBOYCallFollowBytes {
		sequence := append(initial, fb...)
		if findInstructionSequence(bindata, sequence) {
			return true
		}
	}
	return false
}

// NOTE: as far as analysis goes, we may eventually create a large table for all hashes
// to see if your bootloader is "verified": https://github.com/MrBlinky/cathy3k/tree/main/cathy3k
// and for 2k/caterina: https://github.com/MrBlinky/Arduboy/tree/master/cathy

// Compute various important attributes of the given flash data. It could be a
// sketch or a bootloader
func AnalyzeSketch(bindata []byte, bootloader bool) SketchAnalysis {
	result := SketchAnalysis{}
	result.TotalPages = FlashPageCount
	emptyPage := bytes.Repeat([]byte{0xFF}, FlashPageSize)

	for page := 0; page < FlashPageCount; page++ {
		pstart := page * FlashPageSize
		pend := (page + 1) * FlashPageSize
		if len(bindata) > pstart && bytes.Compare(bindata[pstart:pend], emptyPage) != 0 {
			result.TotalPages = page + 1
			if page >= CaterinaStartPage {
				result.OverwritesCaterina = true
			}
			if page >= CathyStartPage {
				result.OverwritesCathy = true
			}
		}
	}

	// TODO: this may/will fail if the data isn't aligned!!
	result.TrimmedData = bindata[:result.TotalPages*FlashPageSize]

	// Use different device detection for bootloader vs sketch. bootloaders always disable the FX, apparently.
	// Sketches may not
	if bootloader {
		if findInstructionSequence(bindata, ARDUBOYFXEnableBytes) && findInstructionSequence(bindata, ARDUBOYFXDisableBytes) {
			result.DetectedDevice = ArduboyFXDeviceKey
		} else if findInstructionSequence(bindata, ARDUBOYMINIEnableBytes) && findInstructionSequence(bindata, ARDUBOYMINIDisableBytes) {
			result.DetectedDevice = ArduboyMiniDeviceKey
		} else {
			// Probably dangerous to assume it's Arduboy but whatever...
			result.DetectedDevice = ArduboyDeviceKey
		}
	} else {
		if findCallRet(bindata, ARDUBOYFXEnableBytes) && findCallRet(bindata, ARDUBOYFXDisableBytes) {
			result.DetectedDevice = ArduboyFXDeviceKey
		} else if findCallRet(bindata, ARDUBOYMINIEnableBytes) && findCallRet(bindata, ARDUBOYMINIDisableBytes) {
			result.DetectedDevice = ArduboyMiniDeviceKey
		} else {
			// Probably dangerous to assume it's Arduboy but whatever...
			result.DetectedDevice = ArduboyDeviceKey
		}
	}

	return result
}

// Read the entire flash memory, including bootloader. This is ironically faster than
// just reading the sketch
func ReadFlash(sercon io.ReadWriter) ([]byte, error) {
	rwep := ReadWriteErrorPass{rw: sercon}
	// Read from address 0
	rwep.WritePass(AddressCommandFlashPage(0))
	var readsingle [1]byte
	rwep.ReadPass(readsingle[:])
	rwep.WritePass(ReadFlashCommand(uint16(FlashSize)))
	var result [FlashSize]byte
	// Read the WHOLE memory (size of FlashSize)
	rwep.ReadPass(result[:])
	return result[:], rwep.err
}

// Read the entire sketch, without the bootloader. Also trims the sketch
func ReadSketch(sercon io.ReadWriter, trim bool) ([]byte, error) {
	// Must get the information about the device
	bootloader, err := GetBootloaderInfo(sercon)
	if err != nil {
		return nil, err
	}
	flash, err := ReadFlash(sercon)
	if err != nil {
		return nil, err
	}
	baseData := flash[:FlashSize-bootloader.Length]
	baseSize := len(baseData)
	if trim {
		trimData := TrimUnused(baseData, FlashPageSize)
		log.Printf("Trimmed sketch removed %d bytes\n", baseSize-len(trimData))
		return trimData, nil
	} else {
		return baseData, nil
	}
}

// Writing a sketch the "right way" is WEIRD because of the intel hex format. The hex file indicates various
// addresses to write data to, not a giant data blob. In theory, you could supply this function with
// hex that writes only every other page, or only some pages in the middle. As such, you must provide
// the raw sketch, not actual binary data, since there might be holes (there most likely aren't).
// This function reads the entire existing sketch area (everything minus the bootloader) into
// memory, applies the hex modifications on top, then writes only the modified pages (smallest
// writable unit) back to the flash memory. We could technically ignore the hex standard and assume
// no sketch will ever have holes and simplify this dramatically, but I wanted this to be as
// correct as possible. Alternatively, to write an "arduboy" sketch program, set fullClear to true
// and you don't have to worry about any weirdness
func WriteHex(sercon io.ReadWriter, rawSketch io.Reader, fullClear bool) ([]byte, []bool, error) {
	// Read the existing sketch area. We will be writing back
	// ONLY the parts that changed (this is what the intel hex format
	// is for) Set some RGB for light indication of which stage we're on
	log.Printf("Reading full sketch + applying hex in-memory\n")
	SetRgbButtonState(sercon, LEDCtrlBtnOff|LEDCtrlBlOn)
	defer ResetRgbButtonState(sercon)
	var sketch []byte
	sketch, err := ReadSketch(sercon, false)
	if err != nil {
		return nil, nil, err
	}
	writtenPages := make([]bool, len(sketch)/FlashPageSize)
	if fullClear {
		// This makes the read USELESS but since it's so fast, we just... do it anyway.
		// If it becomes a problem, properly read the bootloader info and create a fake
		// sketch full of 0xFF
		for i := range sketch {
			sketch[i] = 0xFF
		}
		// Force every page to be written
		for i := range writtenPages {
			writtenPages[i] = true
		}
	}
	log.Printf("Writable flash area is %d bytes (%d pages)\n", len(sketch), len(sketch)/FlashPageSize)
	if len(sketch)%FlashPageSize > 0 {
		return nil, nil, fmt.Errorf("PROGRAM ERROR: sketch area not page aligned! Length: %d", len(sketch))
	}
	// Scan through the hex and the existing sketch, see if it goes beyond
	// the bounds. If it does, it's an error.
	hexmem := gohex.NewMemory()
	//if fullClear {
	//	// Prefill memory with all 0xFF
	//	fullempty := make([]byte, FlashPageSize)
	//	for i := range fullempty {
	//		fullempty[i] = 0xFF
	//	}
	//	for p := 0; p < len(sketch); p += FlashPageSize {
	//		err = hexmem.AddBinary(uint32(p), fullempty)
	//		if err != nil {
	//			return nil, nil, err
	//		}
	//	}
	//}
	err = hexmem.ParseIntelHex(rawSketch)
	if err != nil {
		return nil, nil, err
	}
	for _, segment := range hexmem.GetDataSegments() {
		// Exclusive (the location one past the end of the data)
		endloc := int(segment.Address + uint32(len(segment.Data)))
		if endloc > len(sketch) {
			return nil, nil, fmt.Errorf("Sketch writes outside allowed bounds! At: %d, Max: %d", endloc, len(sketch))
		}
		// Max intel hex length is 255. Just to be safe, set written for all touched pages here
		for p := int(segment.Address); p < endloc; p += FlashPageSize {
			writtenPages[p/FlashPageSize] = true
		}
		copy(sketch[segment.Address:], segment.Data)
	}
	// Now write it back page by page based on which pages have been touched
	log.Printf("Writing ONLY modified sketch pages")
	SetRgbButtonState(sercon, LEDCtrlBtnOff|LEDCtrlRdOn)
	rwep := ReadWriteErrorPass{rw: sercon}
	onebyte := make([]byte, 1)
	for p, write := range writtenPages {
		if write {
			rwep.WritePass(AddressCommandFlashPage(uint16(p)))
			rwep.ReadPass(onebyte)
			rwep.WritePass(WriteFlashCommand(uint16(FlashPageSize)))
			rwep.WritePass(sketch[p*FlashPageSize : (p+1)*FlashPageSize])
			rwep.ReadPass(onebyte)
		}
	}
	if rwep.err != nil {
		return nil, nil, rwep.err
	}
	// Finally, verify the pages. This reads the sketch yet AGAIN into memory
	log.Printf("Validating sketch")
	SetRgbButtonState(sercon, LEDCtrlBtnOff|LEDCtrlGrOn)
	newsketch, err := ReadSketch(sercon, false)
	if err != nil {
		return nil, nil, err
	}
	if len(newsketch) != len(sketch) {
		return nil, nil, fmt.Errorf("Old and new sketch area size doesn't match! Original: %d, New: %d", len(sketch), len(newsketch))
	}
	for p := 0; p < len(newsketch); p += FlashPageSize {
		if !bytes.Equal(sketch[p:p+FlashPageSize], newsketch[p:p+FlashPageSize]) {
			return nil, nil, fmt.Errorf("VALIDATION FAILED: sketch does not match expected at page %d!", p)
		}
	}
	return newsketch, writtenPages, nil
}

// // For the VAST MAJORITY (basically all) sketches, we actually only want the data
// // specified in the sketch and NOTHING else. As such, we actually have a vastly simplified
// // approach to writing, which is simply to create a full sketch of 0xFF, apply the hex on
// // top, then write the entire thing to the arduboy.
// func WriteSketch(sercon io.ReadWriter, rawSketch io.Reader) ([]byte, error) {
// }

// Convert given byte blob to hex. Does NOT modify the data in any way
func BinToHex(data []byte, writer io.Writer) error {
	hexmem := gohex.NewMemory()
	hexmem.SetBinary(0, data)
	funnee := RemoveFirstLines{LinesToIgnore: 1, Writer: writer}
	return hexmem.DumpIntelHex(&funnee, HexLineLength)
}

// Convert hex within given reader to full byte blob. Does NOT modify the
// data in any way (no padding/etc)
func HexToBin(reader io.Reader) ([]byte, error) {
	hexmem := gohex.NewMemory()
	err := hexmem.ParseIntelHex(reader)
	if err != nil {
		return nil, err
	}
	var dataLength uint32 = 0
	for _, segment := range hexmem.GetDataSegments() {
		dataLength = max(dataLength, segment.Address+uint32(len(segment.Data)))
	}
	log.Printf("Computed hex data length is %d\n", dataLength)
	result := make([]byte, dataLength)
	for i := range result {
		result[i] = 0xFF
	}
	for _, segment := range hexmem.GetDataSegments() {
		copy(result[segment.Address:], segment.Data)
	}
	return result, nil
}
