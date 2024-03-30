package arduboy

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

const (
	AnyPortKey           = "any"
	ArduboyDeviceKey     = "Arduboy"
	ArduboyFXDeviceKey   = "ArduboyFX"
	ArduboyMiniDeviceKey = "ArduboyMini"

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
	EepromSize             int = 1024
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
	ID           string
	Capacity     int
	Manufacturer string
}

// Whether this jedec info will fit a flashcart of given size. there are
// caveats to fitting a flashcart (it must end with an empty page and whatever)
func (j *JedecInfo) FitsFlashcart(size int) bool {
	return size+FXPageSize <= j.Capacity
}

// Whether the flashcart of fsize could fit fxdata of dsize at the end.
// Can also specify whether it would fit without block overlap, simplifying
// the writing process (no reads required)
func (j *JedecInfo) ValidateFitsFxData(fsize int, dsize int, noBlockOverlap bool) error {
	rsize := fsize + FXPageSize
	if noBlockOverlap {
		// Flashcart ends IN this block
		endblock := rsize / FXBlockSize
		// FxData starts IN this block
		startblock := (j.Capacity - dsize) / FXBlockSize
		// They only fit if their blocks don't collide. This means that if the
		// flashcart intrudes only 1 page into the final block, absolutely no
		// fxdata could be written, even though the fxdata could be far less
		// than the blocksize (this is the caveat you get with noBlockOverlap)
		if endblock < startblock {
			return nil
		}
		return fmt.Errorf("fxdata block (%d) overlaps flashcart (%d)", startblock, endblock)
	} else {
		if rsize+dsize <= j.Capacity {
			return nil
		}
		return fmt.Errorf("fxdata overlaps: %d + %d > %d", rsize, dsize, j.Capacity)
	}
}

type BasicDeviceInfo struct {
	VidPid       string
	Port         string
	Product      string
	BoardType    string
	IsBootloader bool
}

func (device *BasicDeviceInfo) SmallString() string {
	return fmt.Sprintf("%s:%s(%s)", device.Port, device.VidPid, device.BoardType)
}

type BootloaderInfo struct {
	Device     string
	SoftwareId string
	Startpage  int
	Length     int
	IsCaterina bool
	Version    int
	MD5        string
}

type ExtendedDeviceInfo struct {
	Basic        *BasicDeviceInfo
	Bootloader   *BootloaderInfo
	Jedec        *JedecInfo
	HasFlashcart bool
}

// Construct 'standardized' VID:PID string (the same format python uses, just in case)
func VidPidString(vid string, pid string) string {
	return fmt.Sprintf("VID:PID=%s:%s", vid, pid)
}

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
		return result, nil
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

// Exit the given bootloader
func ExitBootloader(sercon io.ReadWriteCloser) error {
	rwep := ReadWriteErrorPass{rw: sercon}
	onebyte := make([]byte, 1)
	exitstr := [...]byte{'E'}
	rwep.WritePass(exitstr[:])
	rwep.ReadPass(onebyte)
	if rwep.err != nil {
		return rwep.err
	}
	return sercon.Close()
}

// Pull as much bootloader information as possible without overstepping
// into JEDEC or whatever
func GetBootloaderInfo(sercon io.ReadWriter) (*BootloaderInfo, error) {
	var result BootloaderInfo
	var err error
	rwep := ReadWriteErrorPass{rw: sercon}

	// Read software ID
	var sid [7]byte
	rwep.WritePass([]byte("S"))
	rwep.ReadPass(sid[:])
	result.SoftwareId = string(sid[:])

	// Read version
	var version [2]byte
	rwep.WritePass([]byte("V"))
	rwep.ReadPass(version[:])
	result.Version, err = strconv.Atoi(string(version[:]))
	if err != nil {
		return nil, err
	}

	// Figure out if caterina
	if result.Version == 10 {
		rwep.WritePass([]byte("r"))
		var lockbits [1]byte
		rwep.ReadPass(lockbits[:])
		result.IsCaterina = lockbits[0]&0x10 != 0
	} else {
		result.IsCaterina = result.Version < 10
	}

	// TODO: this part NEEDS to be improved!! There's a cathy2K and potentially other bootloaders!
	if result.IsCaterina {
		result.Length = CaterinaTotalSize
	} else {
		result.Length = CathyTotalSize
	}

	result.Startpage = (FlashSize - result.Length) / FlashPageSize

	// Now that we have the length, read the bootloader in its entirety
	var rawbl [CaterinaTotalSize]byte
	rwep.WritePass(AddressCommandFlashPage(uint16(CaterinaStartPage)))
	rwep.ReadPass(rawbl[:1]) // Read just one byte, we don't care what it is apparently
	rwep.WritePass(ReadFlashCommand(uint16(CaterinaTotalSize)))
	rwep.ReadPass(rawbl[:])

	// Cut it down to size before doing work on it. Calculate the hash
	bootloader := rawbl[:result.Length]
	result.MD5 = Md5String(bootloader)

	analysis := AnalyzeSketch(bootloader, true)
	result.Device = analysis.DetectedDevice

	return &result, rwep.err
}

// Ask device for JEDEC info.
// NOTE: this function will block for some time (500ms?) while it verifies the jedec ID!
// (if you ask for it).
func (info *BootloaderInfo) GetJedecInfo(sercon io.ReadWriter, verify bool) (*JedecInfo, error) {
	if info.Version < MinBootloaderWithFlash {
		log.Printf("Bootloader version too low for flashcart support! Need: %d, have: %d\n",
			MinBootloaderWithFlash, info.Version)
		return nil, nil
	}

	rwep := ReadWriteErrorPass{rw: sercon}
	var jedecId1 [3]byte

	rwep.WritePass([]byte("j"))
	rwep.ReadPass(jedecId1[:])

	if verify {
		var jedecId2 [3]byte
		time.Sleep(JedecVerifyWait)
		rwep.WritePass([]byte("j"))
		rwep.ReadPass(jedecId2[:])
		if !bytes.Equal(jedecId1[:], jedecId2[:]) {
			log.Printf("Jedec version producing garbage data, assuming no flashcart!\n")
			return nil, rwep.err
		}
	}

	if bytes.Equal(jedecId1[:], []byte{0, 0, 0}) || bytes.Equal(jedecId1[:], []byte{0xFF, 0xFF, 0xFF}) {
		log.Printf("Jedec version invalid, assuming no flashcart!\n")
		return nil, rwep.err
	}
	result := JedecInfo{
		Capacity: 1 << jedecId1[2],
		ID:       hex.EncodeToString(jedecId1[:]),
	}
	if val, ok := JedecManufacturerKeys[int(jedecId1[0])]; ok {
		result.Manufacturer = val
	} else {
		result.Manufacturer = ""
	}
	return &result, rwep.err
}

// Get extended device info from the given information
func QueryDevice(device *BasicDeviceInfo, sercon io.ReadWriteCloser, verify bool) (*ExtendedDeviceInfo, error) {
	var result ExtendedDeviceInfo
	var err error
	result.Basic = device
	result.Bootloader, err = GetBootloaderInfo(sercon)
	if err != nil {
		return nil, err
	}
	result.Jedec, err = result.Bootloader.GetJedecInfo(sercon, verify)
	if err != nil {
		return nil, err
	}
	result.HasFlashcart = result.Jedec != nil
	return &result, nil
}
