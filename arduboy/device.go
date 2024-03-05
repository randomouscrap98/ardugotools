package arduboy

import (
	"fmt"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"log"
)

// Pulled from Mr.Blinky's Python Utilities:
// https://github.com/MrBlinky/Arduboy-Python-Utilities/blob/main/fxdata-upload.py
var JdecManufacturerKeys = map[int]string{
	0x01: "Spansion",
	0x14: "Cypress",
	0x1C: "EON",
	0x1F: "Adesto(Atmel)",
	0x20: "Micron",
	0x37: "AMIC",
	0x9D: "ISSI",
	0xC2: "General Plus",
	0xC8: "Giga Device",
	0xBF: "Microchip",
	0xEF: "Winbond",
}

const (
	Board_ArduboyLeonardo   = "Arduboy Leonardo"
	Board_ArduboyMicro      = "Arduboy Micro"
	Board_GenuinoMicro      = "Genuino Micro"
	Board_SparkfunMicro     = "Sparkfun Pro Micro 5V"
	Board_AdafruitItsyBitsy = "Adafruit ItsyBitsy 5V"
)

// Pulled from Mr.Blinky's Python Utilities:
// https://github.com/MrBlinky/Arduboy-Python-Utilities/blob/main/fxdata-upload.py
var VidPidTable = map[string]string{
	// Arduboy Leonardo
	"VID:PID=2341:0036": Board_ArduboyLeonardo,
	"VID:PID=2341:8036": Board_ArduboyLeonardo,
	"VID:PID=2A03:0036": Board_ArduboyLeonardo,
	"VID:PID=2A03:8036": Board_ArduboyLeonardo,
	// Arduboy Micro
	"VID:PID=2341:0037": Board_ArduboyMicro,
	"VID:PID=2341:8037": Board_ArduboyMicro,
	"VID:PID=2A03:0037": Board_ArduboyMicro,
	"VID:PID=2A03:8037": Board_ArduboyMicro,
	// Genuino Micro
	"VID:PID=2341:0237": Board_GenuinoMicro,
	"VID:PID=2341:8237": Board_GenuinoMicro,
	// Sparkfun Pro Micro 5V
	"VID:PID=1B4F:9205": Board_SparkfunMicro,
	"VID:PID=1B4F:9206": Board_SparkfunMicro,
	// Adafruit ItsyBitsy 5V
	"VID:PID=239A:000E": Board_AdafruitItsyBitsy,
	"VID:PID=239A:800E": Board_AdafruitItsyBitsy,
}

type JdecInfo struct {
	ID           []byte
	Capacity     int32
	Manufacturer string
}

type DeviceInfo struct {
	VidPid           string
	Port             string
	Product          string
	BoardType        string
	BootloaderDevice string
	Bootloader       bool
	Jdec             JdecInfo
}
