package arduboy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	//"go.bug.st/serial"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

const (
	AnyPortKey = "any"

	ResetToBootloaderWait = 1 * time.Second
	JedecVerifyWait       = 500 * time.Millisecond

	DefaultBaudRate = 57600
	RebootBaudRate  = 1200

	FlashSize              int = 32768
	FlashPageSize          int = 128
	FlashPageCount         int = FlashSize / FlashPageSize
	FXPageSize             int = 256
	FXBlockSize            int = 65536
	FxPagesPerBlock        int = FXBlockSize / FXPageSize
	CaterinaTotalSize      int = 4096
	CaterinaStartPage      int = (FlashSize - CaterinaTotalSize) / FlashPageSize
	CathyTotalSize         int = 3072
	CathyStartPage         int = (FlashSize - CathyTotalSize) / FlashPageSize
	ScreenWidth            int = 128
	ScreenHeight           int = 64
	ScreenBytes            int = ScreenWidth * ScreenHeight / 8
	MinBootloaderWithFlash     = 13
)

// A mapping from identifiers returned from the bootloader to manufacturer strings.
// Pulled from Mr.Blinky's Python Utilities:
// https://github.com/MrBlinky/Arduboy-Python-Utilities/blob/main/fxdata-upload.py
var JedecManufacturerKeys = map[int]string{
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

type JedecInfo struct {
	ID           [3]byte
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
	JedecInfo        *JedecInfo
	HasFlashcart     bool
	BootloaderDevice string
	Version          int
	IsCaterina       bool
	BootloaderLength int
}

// Construct 'standardized' VID:PID string (the same format python uses, just in case)
func VidPidString(vid string, pid string) string {
	return fmt.Sprintf("VID:PID=%s:%s", vid, pid)
}

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
		return nil, nil, fmt.Errorf("Device not found!")
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
		// NOTE: it is OK if the port isn't found again: that's per-operating-system.
		// if you're on Windows, you may be out of luck, but on Linux, chances are high
		// it'll continue to work, so might as well make it work where it can.
		time.Sleep(ResetToBootloaderWait)
		return ConnectWithBootloader(port)
	}
	sercon, err := serial.Open(device.Port, &serial.Mode{BaudRate: DefaultBaudRate})
	if err != nil {
		return nil, nil, err
	}
	return sercon, device, nil
}

// retrieve the version from the bootloader
func GetVersion(sercon io.ReadWriter) (int, error) {
	bcount, err := sercon.Write([]byte("V"))
	if err != nil {
		return 0, err
	}
	if bcount != 1 {
		return 0, fmt.Errorf("Didn't write enough data in GetVersion?")
	}
	var version [2]byte
	bcount, err = sercon.Read(version[:])
	if err != nil {
		return 0, err
	}
	if bcount != 2 {
		return 0, fmt.Errorf("Didn't read enough data in GetVersion?")
	}
	var num int
	num, err = strconv.Atoi(string(version[:]))
	if err != nil {
		return 0, err
	}
	return num, nil
}

// Figure out if the given device (with given pre-read version) is caterina or not
func GetIsCaterina(version int, sercon io.ReadWriter) (bool, error) {
	if version == 10 {
		bcount, err := sercon.Write([]byte("r"))
		if err != nil {
			return false, err
		}
		if bcount != 1 {
			return false, fmt.Errorf("Didn't write enough data in GetIsCaterina?")
		}
		var lockbits [1]byte
		bcount, err = sercon.Read(lockbits[:])
		if err != nil {
			return false, err
		}
		if bcount != 1 {
			return false, fmt.Errorf("Didn't read enough data in GetIsCaterina?")
		}
		return lockbits[0]&0x10 != 0, nil
	}
	return version < 10, nil
}

// figure out the bootloader length based on given information
func (info *ExtendedDeviceInfo) GetBootloaderLength() int {
	// TODO: this function NEEDS to be improved!! There's a cathy2K and potentially other
	// bootloaders!
	if info.IsCaterina {
		return CaterinaTotalSize
	} else {
		return CathyTotalSize
	}
}

// Retrieve the raw Jedec identifier (includes multiple pieces of information)
func getJedecId(sercon io.ReadWriter) ([3]byte, error) {
	var jedecId [3]byte
	bcount, err := sercon.Write([]byte("j"))
	if err != nil {
		return jedecId, err
	}
	if bcount != 1 {
		return jedecId, fmt.Errorf("Didn't write enough data in getJedecId?")
	}
	bcount, err = sercon.Read(jedecId[:])
	if err != nil {
		return jedecId, err
	}
	if bcount != 3 {
		return jedecId, fmt.Errorf("Didn't read enough data in getJedecId?")
	}
	return jedecId, nil
}

func (info *ExtendedDeviceInfo) GetJedecInfo(sercon io.ReadWriter) (*JedecInfo, error) {
	if info.Version < MinBootloaderWithFlash {
		log.Printf("Bootloader version too low for flashcart support! Need: %d, have: %d\n", MinBootloaderWithFlash, info.Version)
		return nil, nil
	}
	var jedecId2 [3]byte
	var result JedecInfo
	var err error
	result.ID, err = getJedecId(sercon)
	if err != nil {
		return nil, err
	}
	time.Sleep(JedecVerifyWait)
	jedecId2, err = getJedecId(sercon)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(result.ID[:], jedecId2[:]) {
		log.Printf("Jedec version producing garbage data, assuming no flashcart!\n")
		return nil, nil
	}
	if bytes.Equal(result.ID[:], []byte{0, 0, 0}) || bytes.Equal(result.ID[:], []byte{0xFF, 0xFF, 0xFF}) {
		log.Printf("Jedec version invalid, assuming no flashcart!\n")
		return nil, nil
	}

	if val, ok := JedecManufacturerKeys[int(result.ID[0])]; ok {
		result.Manufacturer = val
	} else {
		result.Manufacturer = ""
	}

	result.Capacity = 1 << result.ID[2]
	return &result, nil
}

// Get extended device info from the given information
func QueryDevice(device *BasicDeviceInfo, sercon io.ReadWriteCloser) (*ExtendedDeviceInfo, error) {
	var result ExtendedDeviceInfo
	var err error
	result.BasicInfo = *device
	result.Version, err = GetVersion(sercon)
	if err != nil {
		return nil, err
	}
	result.IsCaterina, err = GetIsCaterina(result.Version, sercon)
	if err != nil {
		return nil, err
	}
	result.BootloaderLength = result.GetBootloaderLength()
	result.JedecInfo, err = result.GetJedecInfo(sercon)
	if err != nil {
		return nil, err
	}
	result.HasFlashcart = result.JedecInfo != nil
	return &result, nil
}
