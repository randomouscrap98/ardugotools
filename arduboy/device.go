package arduboy

import (
	"fmt"
	//"go.bug.st/serial"
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

type BasicBoardInfo struct {
	Name         string
	IsBootloader bool
}

// Pulled from Mr.Blinky's Python Utilities:
// https://github.com/MrBlinky/Arduboy-Python-Utilities/blob/main/fxdata-upload.py
var VidPidTable = map[string]BasicBoardInfo{
	// Arduboy Leonardo
	"VID:PID=2341:0036": {Name: Board_ArduboyLeonardo, IsBootloader: true},
	"VID:PID=2341:8036": {Name: Board_ArduboyLeonardo, IsBootloader: false},
	"VID:PID=2A03:0036": {Name: Board_ArduboyLeonardo, IsBootloader: true},
	"VID:PID=2A03:8036": {Name: Board_ArduboyLeonardo, IsBootloader: false},
	// Arduboy Micro
	"VID:PID=2341:0037": {Name: Board_ArduboyMicro, IsBootloader: true},
	"VID:PID=2341:8037": {Name: Board_ArduboyMicro, IsBootloader: false},
	"VID:PID=2A03:0037": {Name: Board_ArduboyMicro, IsBootloader: true},
	"VID:PID=2A03:8037": {Name: Board_ArduboyMicro, IsBootloader: false},
	// Genuino Micro
	"VID:PID=2341:0237": {Name: Board_GenuinoMicro, IsBootloader: true},
	"VID:PID=2341:8237": {Name: Board_GenuinoMicro, IsBootloader: false},
	// Sparkfun Pro Micro 5V
	"VID:PID=1B4F:9205": {Name: Board_SparkfunMicro, IsBootloader: true},
	"VID:PID=1B4F:9206": {Name: Board_SparkfunMicro, IsBootloader: false},
	// Adafruit ItsyBitsy 5V
	"VID:PID=239A:000E": {Name: Board_AdafruitItsyBitsy, IsBootloader: true},
	"VID:PID=239A:800E": {Name: Board_AdafruitItsyBitsy, IsBootloader: false},
}

type JdecInfo struct {
	ID           []byte
	Capacity     int32
	Manufacturer string
}

type BasicDeviceInfo struct {
	VidPid       string
	Port         string
	Product      string
	BoardType    string
	IsBootloader bool
}

type ExtendedDeviceInfo struct {
	BasicInfo        BasicDeviceInfo
	JdecInfo         JdecInfo
	BootloaderDevice string
}

// type DeviceInfo struct {
// 	VidPid           string
// 	Port             string
// 	Product          string
// 	BoardType        string
// 	BootloaderDevice string
// 	Bootloader       bool
// 	Jdec             JdecInfo
// }
//

func VidPidString(vid string, pid string) string {
	return fmt.Sprintf("VID:PID=%s:%s", vid, pid)
}

// First set of functions is for retrieving basic device information, stuff we can
// get without querying the device
func GetBasicDevices() ([]BasicDeviceInfo, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}
	result := make([]BasicDeviceInfo, 0)
	if len(ports) == 0 {
		log.Println("No serial ports found!")
		return result, err
	}
	for _, port := range ports {
		if port.IsUSB {
			var vidpid = fmt.Sprintf("VID:PID=%s:%s", port.VID, port.PID)
			foundDevice := false
			// See if the device's VIDPID is in the table
			for key, boardinfo := range VidPidTable {
				// We may have dummy values; see isBootLoader later
				if key == vidpid {
					result = append(result, BasicDeviceInfo{
						VidPid:       vidpid,
						BoardType:    boardinfo.Name,
						IsBootloader: boardinfo.IsBootloader,
						Product:      port.Product,
						Port:         port.Name,
					})
					foundDevice = true
					break
				}
			}
			if !foundDevice {
				log.Println("Non-arduboy device on port ", port.Name, ": ", vidpid)
			}
		} else {
			log.Println("Port not USB: ", port.Name)
		}
	}
	return result, nil
}
