package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/alecthomas/kong"
	"github.com/randomouscrap98/ardugotools/arduboy"
)

// Quick way to fail on error, since most commands are "doing" something on
// behalf of something else.
func fatalIfErr(subject string, doing string, err error) {
	if err != nil {
		log.Fatalf("%s - Couldn't %s: %s", subject, doing, err)
	}
}

func connectWithBootloader(device string) (io.ReadWriteCloser, *arduboy.BasicDeviceInfo) {
	sercon, d, err := arduboy.ConnectWithBootloader(device)
	fatalIfErr(device, "connect", err)
	log.Printf("Initial contact with %s, set to bootloader mode\n", d.SmallString())
	return sercon, d
}

func mustHaveFlashcart(sercon io.ReadWriteCloser, device *arduboy.BasicDeviceInfo) *arduboy.ExtendedDeviceInfo {
	extdata, err := arduboy.QueryDevice(device, sercon, false)
	fatalIfErr(device.Port, "check for flashcart", err)
	log.Printf("Device %s has flashcart: %v\n", device.SmallString(), extdata.Jedec)
	if extdata.Jedec == nil {
		log.Fatalf("Device %s doesn't seem to have a flashcart!\n", extdata.Bootloader.Device)
	}
	return extdata
}

// **********************************
// *       DEVICES COMMANDS         *
// **********************************

// Scan command
type ScanCmd struct {
}

func (c *ScanCmd) Run() error {
	devices, err := arduboy.GetBasicDevices()
	fatalIfErr("scan", "pull devices", err)
	log.Printf("Scan found %d viable devices\n", len(devices))
	PrintJson(devices)
	return nil
}

// Query command
type QueryCmd struct {
	Device string `arg:"" help:"The system device to check (use 'any' for first)"`
}

func (c *QueryCmd) Run() error {
	sercon, d := connectWithBootloader(c.Device)
	extdata, err := arduboy.QueryDevice(d, sercon, true)
	fatalIfErr(c.Device, "query device information", err)
	log.Printf("Device %s is probably a %s\n", d.SmallString(), extdata.Bootloader.Device)
	PrintJson(extdata)
	return nil
}

// **********************************
// *       SKETCH COMMANDS          *
// **********************************

// Sketch read command
type SketchReadCmd struct {
	Device  string `arg:"" help:"The system device to read from (use 'any' for first)"`
	Outfile string `short:"o"`
}

func (c *SketchReadCmd) Run() error {
	// Read sketch First
	sercon, d := connectWithBootloader(c.Device)
	sketch, err := arduboy.ReadSketch(sercon)
	fatalIfErr(c.Device, "read sketch", err)
	log.Printf("Read %d bytes from %s\n", len(sketch), d.SmallString())
	hash := arduboy.Md5String(sketch)
	// Figure out save location
	if c.Outfile == "" {
		c.Outfile = fmt.Sprintf("%s_%s.hex", hash, FileSafeDateTime())
	}
	// Open and save file
	file, err := os.Create(c.Outfile)
	fatalIfErr(c.Outfile, "open file for writing", err)
	defer file.Close()
	err = arduboy.BinToHex(sketch, file)
	fatalIfErr(c.Outfile, "convert sketch to hex", err)
	log.Printf("Wrote sketch to file %s\n", c.Outfile)
	// Return data about the save
	result := make(map[string]interface{})
	result["Filename"] = c.Outfile
	result["MD5"] = hash
	PrintJson(result)
	return nil
}

// **********************************
// *       EEPROM COMMANDS          *
// **********************************

// Eeprom read command
type EepromReadCmd struct {
	Device  string `arg:"" help:"The system device to read from (use 'any' for first)"`
	Outfile string `short:"o"`
}

func (c *EepromReadCmd) Run() error {
	// Read eeprom first
	sercon, d := connectWithBootloader(c.Device)
	eeprom, err := arduboy.ReadEeprom(sercon)
	fatalIfErr(c.Device, "read eeprom", err)
	log.Printf("Read %d bytes from %s (full eeprom)\n", len(eeprom), d.SmallString())
	hash := arduboy.Md5String(eeprom)
	// Figure out save location
	if c.Outfile == "" {
		c.Outfile = fmt.Sprintf("eeprom_%s.bin", FileSafeDateTime())
	}
	// Open and save file
	file, err := os.Create(c.Outfile)
	fatalIfErr(c.Outfile, "open file for writing", err)
	defer file.Close()
	num, err := file.Write(eeprom)
	if num != len(eeprom) {
		log.Fatalf("Didn't write full file! This is strange!")
	}
	fatalIfErr(c.Outfile, "write eeprom to file", err)
	log.Printf("Wrote eeprom to file %s\n", c.Outfile)
	// Return data about the save
	result := make(map[string]interface{})
	result["Filename"] = c.Outfile
	result["MD5"] = hash
	PrintJson(result)
	return nil
}

// Eeprom write command
type EepromWriteCmd struct {
	Device string `arg:"" help:"The system device to read from (use 'any' for first)"`
	Infile string `short:"i"`
}

func (c *EepromWriteCmd) Run() error {
	sercon, d := connectWithBootloader(c.Device)
	// Go find the file first
	if c.Infile == "" {
		c.Infile = "eeprom.bin"
	}
	eeprom, err := os.ReadFile(c.Infile)
	fatalIfErr(c.Device, "read file", err)
	log.Printf("Read %d bytes from file %s\n", len(eeprom), c.Infile)
	// Now write the eeprom
	err = arduboy.WriteEeprom(sercon, eeprom)
	fatalIfErr(c.Device, "write eeprom", err)
	log.Printf("Wrote %d bytes to %s (full eeprom)\n", len(eeprom), d.SmallString())
	hash := arduboy.Md5String(eeprom)
	// Return data about the eeprom (does this even matter?)
	result := make(map[string]interface{})
	result["Filename"] = c.Infile
	result["MD5"] = hash
	PrintJson(result)
	return nil
}

// Eeprom delete command
type EepromDeleteCmd struct {
	Device string `arg:"" help:"The system device to read from (use 'any' for first)"`
}

func (c *EepromDeleteCmd) Run() error {
	sercon, d := connectWithBootloader(c.Device)
	err := arduboy.DeleteEeprom(sercon)
	fatalIfErr(c.Device, "delete eeprom", err)
	log.Printf("Deleted eeprom on %s\n", d.SmallString())
	return nil
}

// **********************************
// *      FLASHCART COMMANDS        *
// **********************************

// Flashcart scan command
type FlashcartScanCmd struct {
	Device string `arg:"" help:"The system device to read from (use 'any' for first)"`
	Html   bool   `help:"Generate as html instead"`
	Images bool   `help:"Pull images (takes 4 times as long)"`
}

func (c *FlashcartScanCmd) Run() error {
	sercon, d := connectWithBootloader(c.Device)
	extd := mustHaveFlashcart(sercon, d)
	result, err := arduboy.ScanFlashcartMeta(sercon, c.Images)
	programs := 0
	for _, c := range result {
		programs += len(c.Slots)
	}
	fatalIfErr(c.Device, "scan flashcart (basic)", err)
	log.Printf("Scanned %d categories, %d programs from flashcart on %s\n", len(result), programs, d.SmallString())
	if c.Html {
		err = arduboy.RenderFlashcartMeta(result, extd, os.Stdout)
		fatalIfErr(c.Device, "render flashcart into HTML", err)
	} else {
		PrintJson(result)
	}
	return nil
}

// **********************************
// *    ALL TOGETHER COMMANDS       *
// **********************************

var cli struct {
	Scan   ScanCmd  `cmd:"" help:"Search for Arduboys and return basic information on them"`
	Query  QueryCmd `cmd:"" help:"Get deeper information about a particular Arduboy"`
	Sketch struct {
		Read SketchReadCmd `cmd:"" help:"Read just the sketch portion of flash, saved as a .hex file"`
	} `cmd:"" help:"Perform actions on the builtin flash or related to sketch files"`
	Eeprom struct {
		Read   EepromReadCmd   `cmd:"" help:"Read entire eeprom, saved as a .bin file"`
		Write  EepromWriteCmd  `cmd:"" help:"Write data to eeprom"`
		Delete EepromDeleteCmd `cmd:"" help:"Reset entire eeprom"`
	} `cmd:"" help:"Perform actions on the builtin eeprom (save area)"`
	Flashcart struct {
		Scan FlashcartScanCmd `cmd:"" help:"Scan flashcart and return categories/games"`
	} `cmd:"" help:"Perform actions on the 'external' flashcart (FX/Mini/etc)"`
}

func main() {
	ctx := kong.Parse(&cli)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
