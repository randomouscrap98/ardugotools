package arduboy

import (
	"crypto/md5"
	"encoding/hex"
	"io"
)

// Produce the command for setting the address before reading (page aligned)
func AddressCommandPage(page uint16) []byte {
	return []byte{byte('A'), byte(page >> 2), byte((page & 3) << 6)}
}

// Produce the command for reading some amount from the current address
func ReadFlashCommand(length uint16) []byte {
	return []byte{byte('g'), byte(length >> 8), byte(length & 255), byte('F')}
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

// The hex writer library writes some record indicating an extended address
// mode which Arduboy CLEARLY doesn't need nor want, and so we get rid of it.
// By force.
type FunnyHexFixer struct {
	FirstLineRead bool
	Writer        io.Writer
}

// Skip everything up to and including the first newline
func (fhf *FunnyHexFixer) Write(data []byte) (int, error) {
	if fhf.FirstLineRead {
		return fhf.Writer.Write(data)
	}
	for i, b := range data {
		if b == byte('\n') {
			fhf.FirstLineRead = true
			return fhf.Write(data[i+1:])
		}
	}
	return len(data), nil
}
