package arduboy

import (
	"fmt"
	"io"
)

// Read the entire flash memory, including bootloader. This is ironically faster than
// just reading the sketch
func ReadEeprom(sercon io.ReadWriter) ([]byte, error) {
	rwep := ReadWriteErrorPass{rw: sercon}
	// Read from address 0
	rwep.WritePass(AddressCommandFlashPage(0))
	onebyte := make([]byte, 1)
	result := make([]byte, EepromSize)
	rwep.ReadPass(onebyte)
	rwep.WritePass(ReadEepromCommand(uint16(EepromSize)))
	// Read the WHOLE memory (size of eeprom)
	rwep.ReadPass(result)
	return result, rwep.err
}

// Write the given eeprom to the device. This may take a while
func WriteEeprom(sercon io.ReadWriter, data []byte) error {
	if len(data) != EepromSize {
		return fmt.Errorf("Wrong data size for eeprom! Expect: %d", EepromSize)
	}
	rwep := ReadWriteErrorPass{rw: sercon}
	// This eeprom thing actually takes a while. Turn the light to yellow,
	// then defer the reset
	SetRgbButtonState(sercon, LEDCtrlBtnOff|LEDCtrlRdOn|LEDCtrlGrOn)
	defer ResetRgbButtonState(sercon)
	// Write to address 0
	onebyte := make([]byte, 1)
	rwep.WritePass(AddressCommandFlashPage(0))
	rwep.ReadPass(onebyte)
	rwep.WritePass(WriteEepromCommand(uint16(EepromSize)))
	// Write the WHOLE memory (size of eeprom)
	rwep.WritePass(data)
	rwep.ReadPass(onebyte)
	return rwep.err
}

// Delete the entire eeprom
func DeleteEeprom(sercon io.ReadWriter) error {
	eeprom := make([]byte, EepromSize)
	for i := range eeprom {
		eeprom[i] = 0xFF
	}
	return WriteEeprom(sercon, eeprom)
}
