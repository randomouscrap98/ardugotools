package arduboy

import (
	"bytes"
	"log"
	// "go.bug.st/serial"
	// "go.bug.st/serial/enumerator"
	"io"
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
	rwep.WritePass(AddressCommandPage(0))
	var readsingle [1]byte
	rwep.ReadPass(readsingle[:])
	rwep.WritePass(ReadFlashCommand(uint16(FlashSize)))
	var result [FlashSize]byte
	// Read the WHOLE memory (size of FlashSize)
	rwep.ReadPass(result[:])
	return result[:], rwep.err
}

// Read the entire sketch, without the bootloader. Also trims the sketch
func ReadSketch(sercon io.ReadWriter) ([]byte, error) {
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
	trimData := TrimUnused(baseData, FlashPageSize)
	log.Printf("Trimmed sketch removed %d bytes\n", baseSize-len(trimData))
	return trimData, nil
}
