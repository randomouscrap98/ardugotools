package arduboy

import (
	"errors"
	"fmt"
	"io"
	"time"

	//"go.bug.st/serial"
	"log"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

const AnyPortKey = "any"

const (
	DefaultBaudRate = 57600
	RebootBaudRate  = 1200
)

// A mapping from identifiers returned from the bootloader to manufacturer strings.
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

// A mapping from VID/PID values to basic information about the board.
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

// Construct 'standardized' VID:PID string (the same format python uses, just in case)
func VidPidString(vid string, pid string) string {
	return fmt.Sprintf("VID:PID=%s:%s", vid, pid)
}

// Given a single port info, parse as much as you can. Returns error
// if port isn't arduboy
/* func parsePortInfo(port *enumerator.PortDetails) BasicDeviceInfo, error {
		if port.IsUSB {
			var vidpid = fmt.Sprintf("VID:PID=%s:%s", port.VID, port.PID)
			// See if the device's VIDPID is in the table
			for key, boardinfo := range VidPidTable {
				// We may have dummy values; see isBootLoader later
				if key == vidpid {
					return BasicDeviceInfo{
						VidPid:       vidpid,
						BoardType:    boardinfo.Name,
						IsBootloader: boardinfo.IsBootloader,
						Product:      port.Product,
						Port:         port.Name,
					}), nil
				}
			}
			if !foundDevice {
				log.Println("Non-arduboy device on port ", port.Name, ": ", vidpid)
			}
		} else {
			log.Println("Port not USB: ", port.Name)
		}
} */

// First set of functions is for retrieving basic device information, stuff we can
// get without querying the device

// Retrieve a list of all connected arduboys and any information that can be parsed
// without actually connected to the ports
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

// Connect to given port and force bootloader. Accepts "any" as a special port identifier,
// will connect to "first" connection found. If exact port is given and no bootloader
// specified, will reboot device and NOT connect, since it is not always possible to
// reconnect on the same port. If "any" given, will attempt a reconnect after 2 seconds
func ConnectWithBootloader(port string) (io.ReadWriteCloser, *BasicDeviceInfo, error) {
	// To make life WAY easier, just query for all arduboys again (even though the user
	// may have already done this)
	devices, err := GetBasicDevices()
	if err != nil {
		return nil, nil, err
	}
	// Scan for device in connected devices
	var device *BasicDeviceInfo = nil
	if port == AnyPortKey {
		if len(devices) > 0 {
			device = &devices[0]
		}
	} else {
		for _, d := range devices {
			if d.Port == port {
				device = &d
				break
			}
		}
	}
	if device == nil {
		return nil, nil, errors.New("Device not found!")
	}
	// Now, check if bootloader. If not, have to reconnect and try again
	if !device.IsBootloader {
		log.Println("Attempting to reset device ", device.Port, " (not bootloader)")
		sercon, err := serial.Open(device.Port, &serial.Mode{BaudRate: RebootBaudRate})
		if err != nil {
			return nil, nil, err
		}
		err = sercon.Close()
		if err != nil {
			return nil, nil, err
		}
		if port == AnyPortKey {
			time.Sleep(2 * time.Second)
			return ConnectWithBootloader(port)
		} else {
			return nil, nil, errors.New(fmt.Sprintf("Device %s not in bootloader mode, reset", device.Port))
		}
	}
	sercon, err := serial.Open(device.Port, &serial.Mode{BaudRate: DefaultBaudRate})
	if err != nil {
		return nil, nil, err
	}
	return sercon, device, nil
}
