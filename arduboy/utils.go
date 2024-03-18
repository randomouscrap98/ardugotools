package arduboy

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"io"
)

const (
	LEDCtrlBtnOff = 0x80
	LEDCtrlRGB    = 0x40
	LEDCtrlRxTx   = 0x20
	LEDCtrlRxOn   = 0x10
	LEDCtrlTxOn   = 0x08
	LEDCtrlGrOn   = 0x04
	LEDCtrlRdOn   = 0x02
	LEDCtrlBlOn   = 0x01
)

// Produce the command for setting the address before reading (raw address)
func AddressCommandRaw(address uint16) []byte {
	return []byte{byte('A'), byte(address >> 8), byte(address & 0xFF)}
}

// Produce the command for setting the flash address based on true byte offset
func AddressCommandFlashAddress(address uint16) []byte {
	return AddressCommandRaw(address >> 1) // I don't quite know why it's 2-byte-word aligned
}

// Produce the command for setting the address before reading (page aligned)
func AddressCommandFlashPage(page uint16) []byte {
	return AddressCommandFlashAddress(page * uint16(FlashPageSize))
}

// Produce the command for setting the address before reading flashcart data (flashcart page aligned)
func AddressCommandFlashcartPage(page uint16) []byte {
	return AddressCommandRaw(page) //No change
}

func ReadWriteCommandRaw(mode rune, length uint16, device rune) []byte {
	return []byte{byte(mode), byte(length >> 8), byte(length & 0xFF), byte(device)}
}

// Produce the command for reading some amount from the current address
func ReadFlashCommand(length uint16) []byte {
	return ReadWriteCommandRaw('g', length, 'F')
}

// Produce command for reading an amount from the flashcart
func ReadFlashcartCommand(length uint16) []byte {
	return ReadWriteCommandRaw('g', length, 'C')
}

func WriteFlashCommand(length uint16) []byte {
	return ReadWriteCommandRaw('B', length, 'F')
}

// Produce command for reading an amount from eeprom (probably the whole thing though)
func ReadEepromCommand(length uint16) []byte {
	return ReadWriteCommandRaw('g', length, 'E')
}

func WriteEepromCommand(length uint16) []byte {
	return ReadWriteCommandRaw('B', length, 'E')
}

// This is bad but whatever: globally disable rgb lights
var enableRgb = true

func SetRgbEnabledGlobal(enabled bool) {
	enableRgb = enabled
}

func RgbButtonCommandRaw(data uint8) []byte {
	if !enableRgb {
		data = data & LEDCtrlBtnOff
	}
	return []byte{byte('x'), data}
}

func RgbButtonCommand(data uint8) []byte {
	if (data & (LEDCtrlRdOn | LEDCtrlGrOn | LEDCtrlBlOn)) > 0 {
		data |= LEDCtrlRGB
	}
	if (data & (LEDCtrlRxOn | LEDCtrlTxOn)) > 0 {
		data |= LEDCtrlRxTx
	}
	return RgbButtonCommandRaw(data)
}

func SetRgbButtonState(sercon io.ReadWriter, state uint8) error {
	var sb [1]byte
	rwep := ReadWriteErrorPass{rw: sercon}
	rwep.WritePass(RgbButtonCommand(state))
	rwep.ReadPass(sb[:])
	return rwep.err
}

func ResetRgbButtonState(sercon io.ReadWriter) error {
	return SetRgbButtonState(sercon, 0)
	// rwep.WritePass(RgbButtonCommandRaw(LEDCtrlRGB))
}

// Remove unused sections from the end of the byte array. In these files, sections
// of 0xFF represent unused data.
func TrimUnused(data []byte, blocksize int) []byte {
	unusedLength := 0
	dlen := len(data)
	for ; unusedLength < dlen; unusedLength++ {
		if data[dlen-1-unusedLength] != 0xFF {
			break
		}
	}

	// Now just trim unused length off the end, but aligned to the smallest blocksize
	trim := (unusedLength / blocksize) * blocksize
	return data[:dlen-trim]
}

// Produce an md5 string from given data (a simple shortcut)
func Md5String(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// A writer wrapper which removes the first few lines of output
type RemoveFirstLines struct {
	LinesToIgnore int
	Writer        io.Writer
}

// Skip everything up to and including the first N newlines
func (fhf *RemoveFirstLines) Write(data []byte) (int, error) {
	if fhf.LinesToIgnore <= 0 {
		// Ready to write
		return fhf.Writer.Write(data)
	}
	// Consume bytes until we reach the requisite number of newlines
	for i, b := range data {
		if b == byte('\n') {
			fhf.LinesToIgnore -= 1
			written, err := fhf.Write(data[i+1:])
			written += (i + 1)
			return written, err
		}
	}
	return len(data), nil
}

// Read a 2 byte value in the middle of data
func Get2ByteValue(data []byte, index int) uint16 {
	return binary.BigEndian.Uint16(data[index : index+2])
}

// Write a 2 byte value directly into the middle of data
func Write2ByteValue(value uint16, data []byte, index int) {
	data[index] = byte(value >> 8)
	data[index+1] = byte(value & 0xFF)
}

// Parse as many null-terminated strings as possible out of
// the data. Useful for the header "metadata"
func ParseStringArray(data []byte) []string {
	result := make([]string, 0)
	for len(data) > 0 {
		next := bytes.IndexByte(data, 0)
		if next == -1 {
			break
		}
		result = append(result, string(data[:next]))
		data = data[next+1:]
	}
	return result
}

// Fill the given data byte buffer with as much of the strings contained within
// the strings parameter. Returns the total amount of strings written and the
// amount truncated from the last written string
func FillStringArray(strings []string, data []byte) (int, int) {
	for i, s := range strings {
		// Copy the full strings (or as much as it can) into the remaining data section.
		slen := copy(data, s)
		dnext := slen
		if dnext < len(data) {
			// Add the null terminator if there's room
			data[dnext] = 0
			dnext++
		}
		if dnext < len(data) {
			// Move to the next string if there's room (including the null terminator)
			data = data[dnext:]
		} else {
			// We ran out of room on this given string
			return i + 1, len(s) - slen
		}
	}
	// We were able to write everything
	return len(strings), 0
}
