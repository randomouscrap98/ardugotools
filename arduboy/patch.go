package arduboy

import (
	"bytes"
	"fmt"
)

const (
	LCDBOOTPROGRAM  = "\xD5\xF0\x8D\x14\xA1\xC8\x81\xCF\xD9\xF1\xAF\x20\x00"
	MENUBUTTONPATCH = "\x0f\x92\x0f\xb6\x8f\x93\x9f\x93\xef\x93\xff\x93\x80\x91\xcc\x01" +
		"\x8d\x5f\x8d\x37\x08\xf0\x8d\x57\x80\x93\xcc\x01\xe2\xe4\xf3\xe0" +
		"\x80\x81\x8e\x4f\x80\x83\x91\x81\x9f\x4f\x91\x83\x82\x81\x8f\x4f" +
		"\x82\x83\x83\x81\x8f\x4f\x83\x83\xed\xec\xf1\xe0\x80\x81\x8f\x5f" +
		"\x80\x83\x81\x81\x8f\x4f\x81\x83\x82\x81\x8f\x4f\x82\x83\x83\x81" +
		"\x8f\x4f\x83\x83\x8f\xb1\x8f\x60\x66\x99\x1c\x9b\x88\x27\x8f\x36" +
		"\x81\xf4\x80\x91\xFF\x0A\x98\x1b\x96\x30\x68\xf0\xe0\xe0\xf8\xe0" +
		"\x87\xe7\x80\x83\x81\x83\x88\xe1\x80\x93\x60\x00\xf0\x93\x60\x00" +
		"\xff\xcf\x90\x93\xFF\x0A\xff\x91\xef\x91\x9f\x91\x8f\x91\x0f\xbe" +
		"\x0f\x90\x18\x95"

	RET_INSTRUCTION  = "\x08\x95"
	RETI_INSTRUCTION = "\x18\x95"

	CONTRAST_NOCHANGE = -1
	CONTRAST_NORMAL   = 0xCF
	CONTRAST_DIM      = 0x7F
	CONTRAST_DIMMER   = 0x2F
	CONTRAST_DIMMEST  = 0x00
	CONTRAST_HIGHEST  = 0xFF

	MBP_fract_lds    = 14
	MBP_fract_sts    = 26
	MBP_millis_r30   = 28
	MBP_millis_r31   = 30
	MBP_overflow_r30 = 56
	MBP_overflow_r31 = 58
)

// Directly modify the given program so that it allows resetting to the
// bootloader with up and down
func PatchMenuButtons(program []byte) (bool, string) {
	if len(program) < 256 {
		return false, "Program too short"
	}

	var vector_23 int = (int(program[0x5E]) << 1) | (int(program[0x5F]) << 9) //ISR timer0 vector addr
	var p int = vector_23
	l := 0
	lds := 0
	branch := 0
	timer0_millis := 0
	timer0_fract := 0
	timer0_overflow_count := 0

	for p < (len(program) - 2) {
		p += 2 // handle 2 byte instructions
		if string(program[p-2:p]) == RET_INSTRUCTION {
			l = -1
			break
		}
		if (program[p-1]&0xFC) == 0xF4 && (program[p-2]&0x07) == 0x00 { // brcc instruction may jump beyond reti
			branch = (int(program[p-1]&0x03) << 6) + (int(program[p-2]&0xf8) >> 2)
			if branch < 128 {
				branch = p + branch
			} else {
				branch = p - 256 + branch
			}
		}
		if string(program[p-2:p]) == RETI_INSTRUCTION {
			l = p - vector_23
			if p > branch { // there was no branch beyond reti instruction
				break
			}
		}
		if l != 0 { // branched beyond reti, look for rjmp instruction
			if (program[p-1] & 0xF0) == 0xC0 {
				l = p - vector_23
				break
			}
		}
		// handle 4 byte instructions
		if (program[p-1]&0xFE) == 0x90 && (program[p-2]&0x0F) == 0x00 { // lds instruction
			lds += 1
			if lds == 1 {
				timer0_millis = int(program[p]) | (int(program[p+1]) << 8)
			} else if lds == 5 {
				timer0_fract = int(program[p]) | (int(program[p+1]) << 8)
			} else if lds == 6 {
				timer0_overflow_count = int(program[p]) | (int(program[p+1]) << 8)
			}
			p += 2
		}
		if (program[p-1]&0xFE) == 0x92 && (program[p-2]&0x0F) == 0x00 { // sts instruction
			p += 2
		}
	}

	if l == -1 {
		return false, "No menu patch applied. ISR contains subroutine."
	} else if l < len(MENUBUTTONPATCH) {
		return false, fmt.Sprintf("No menu patch applied. ISR size too small (%d bytes)", l)
	} else if timer0_millis == 0 || timer0_fract == 0 || timer0_overflow_count == 0 {
		return false, "No menu patch applied. Custom ISR in use."
	} else {
		// patch the new ISR code with 'hold UP + DOWN for 2 seconds to start bootloader menu' feature
		copied := copy(program[vector_23:], []byte(MENUBUTTONPATCH))
		if copied != len(MENUBUTTONPATCH) {
			return false, "ARDUGOTOOLS PROGRAM ERROR: didn't copy whole menu patch!"
		}
		//program[vector_23 : vector_23+len(MENUBUTTONPATCH)] = MENUBUTTONPATCH
		// fix timer variables
		program[vector_23+MBP_fract_lds+0] = byte(timer0_fract & 0xFF)
		program[vector_23+MBP_fract_lds+1] = byte(timer0_fract >> 8)
		program[vector_23+MBP_fract_sts+0] = byte(timer0_fract & 0xFF)
		program[vector_23+MBP_fract_sts+1] = byte(timer0_fract >> 8)
		program[vector_23+MBP_millis_r30+0] = byte(0xE0 | (timer0_millis>>0)&0x0F)
		program[vector_23+MBP_millis_r30+1] = byte(0xE0 | (timer0_millis>>4)&0x0F)
		program[vector_23+MBP_millis_r31+0] = byte(0xF0 | (timer0_millis>>8)&0x0F)
		program[vector_23+MBP_millis_r31+1] = byte(0xE0 | (timer0_millis>>12)&0x0F)
		program[vector_23+MBP_overflow_r30+0] = byte(0xE0 | (timer0_overflow_count>>0)&0x0F)
		program[vector_23+MBP_overflow_r30+1] = byte(0xE0 | (timer0_overflow_count>>4)&0x0F)
		program[vector_23+MBP_overflow_r31+0] = byte(0xF0 | (timer0_overflow_count>>8)&0x0F)
		program[vector_23+MBP_overflow_r31+1] = byte(0xE0 | (timer0_overflow_count>>12)&0x0F)
		return true, "Menu patch applied"
	}
}

// Apply a combination of screen patches to the given program
func PatchScreen(flashdata []byte, ssd1309 bool, contrast int) int {
	//logging.debug(f"Patching screen data: ssd1309={ssd1309}, contrast={contrast}")
	lcdBootProgram_addr := 0
	found := 0
	for lcdBootProgram_addr >= 0 {
		lcdBootProgram_addr = bytes.Index(flashdata[lcdBootProgram_addr:], []byte(LCDBOOTPROGRAM[:7]))
		//flashdata.find(LCDBOOTPROGRAM[:7], lcdBootProgram_addr)
		if lcdBootProgram_addr >= 0 && string(flashdata[lcdBootProgram_addr+8:lcdBootProgram_addr+13]) == LCDBOOTPROGRAM[8:] {
			found += 1
			if ssd1309 {
				flashdata[lcdBootProgram_addr+2] = 0xE3
				flashdata[lcdBootProgram_addr+3] = 0xE3
			}
			if contrast >= 0 {
				flashdata[lcdBootProgram_addr+7] = byte(contrast)
			}
			lcdBootProgram_addr += 8
		}
	}
	return found
}

// Given binary data, patch EVERY instance of wrong LED polarity for Micro
// Taken directly from https://github.com/MrBlinky/Arduboy-Python-Utilities/blob/main/uploader.py
func PatchMicroLED(flashdata []byte) {
	for i := 0; i < FlashSize-4; i += 2 { //range(0,FLASH_SIZE-4,2) {
		if string(flashdata[i:i+2]) == "\x28\x98" { // RXLED1
			flashdata[i+1] = 0x9a
		} else if string(flashdata[i:i+2]) == "\x28\x9a" { // RXLED0
			flashdata[i+1] = 0x98
		} else if string(flashdata[i:i+2]) == "\x5d\x98" { // TXLED1
			flashdata[i+1] = 0x9a
		} else if string(flashdata[i:i+2]) == "\x5d\x9a" { // TXLED0
			flashdata[i+1] = 0x98
		} else if string(flashdata[i:i+4]) == "\x81\xef\x85\xb9" { // Arduboy core init RXLED port
			flashdata[i] = 0x80
		} else if string(flashdata[i:i+4]) == "\x84\xe2\x8b\xb9" { // Arduboy core init TXLED port
			flashdata[i+1] = 0xE0
		}
	}
}
