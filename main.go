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
	defer sercon.Close()
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
	Outfile string `type:"path" short:"o"`
}

func (c *SketchReadCmd) Run() error {
	// Figure out save location
	if c.Outfile == "" {
		c.Outfile = fmt.Sprintf("sketch_%s.hex", FileSafeDateTime())
	}
	// Read sketch
	sercon, d := connectWithBootloader(c.Device)
	defer sercon.Close()
	sketch, err := arduboy.ReadSketch(sercon, true)
	fatalIfErr(c.Device, "read sketch", err)
	log.Printf("Read %d bytes from %s\n", len(sketch), d.SmallString())
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
	result["MD5"] = arduboy.Md5String(sketch)
	result["SketchLength"] = len(sketch)
	PrintJson(result)
	return nil
}

// Raw Hex write command
type RawHexWriteCmd struct {
	Device string `arg:"" help:"The system device to write to (use 'any' for first)"`
	Infile string `type:"existingfile" short:"i" help:"File to load hex from"`
	Runnow bool   `help:"Run sketch immediately"`
}

func (c *RawHexWriteCmd) Run() error {
	sercon, d := connectWithBootloader(c.Device)
	defer sercon.Close()
	// Go find the file first
	if c.Infile == "" {
		c.Infile = "sketch.hex"
	}
	sketchRaw, err := os.Open(c.Infile)
	fatalIfErr(c.Device, "read file", err)
	// Now write the sketch. This includes validation steps
	sketch, writtenPages, err := arduboy.WriteHex(sercon, sketchRaw, false)
	fatalIfErr(c.Device, "write raw hex", err)
	// Figure out some data to give back to the user about the sketch write
	numwritten := 0
	lastWritten := -1
	contiguous := true
	for i, w := range writtenPages {
		if w {
			numwritten++
			if lastWritten >= 0 && lastWritten != i-1 {
				contiguous = false
			}
			lastWritten = i
		}
	}
	log.Printf("Wrote %d pages to %s\n", numwritten, d.SmallString())
	if c.Runnow {
		arduboy.ExitBootloader(sercon)
	}
	hash := arduboy.Md5String(sketch)
	// Return data about the eeprom (does this even matter?)
	result := make(map[string]interface{})
	result["Filename"] = c.Infile
	result["PagesWritten"] = numwritten
	result["Contiguous"] = contiguous
	result["SketchLength"] = numwritten * arduboy.FlashPageSize
	result["UsableFlashLength"] = len(sketch)
	result["UsableFlashMD5"] = hash
	PrintJson(result)
	return nil
}

// Sketch write command (use this one)
type SketchWriteCmd struct {
	Device string `arg:"" help:"The system device to write to (use 'any' for first)"`
	Infile string `type:"existingfile" short:"i" help:"File to load hex from"`
	Runnow bool   `help:"Run sketch immediately"`
}

func (c *SketchWriteCmd) Run() error {
	sercon, d := connectWithBootloader(c.Device)
	defer sercon.Close()
	// Go find the file first
	if c.Infile == "" {
		c.Infile = "sketch.hex"
	}
	sketchRaw, err := os.Open(c.Infile)
	fatalIfErr(c.Device, "read file", err)
	// Now write the sketch. This includes validation steps
	sketch, writtenPages, err := arduboy.WriteHex(sercon, sketchRaw, true)
	fatalIfErr(c.Device, "write raw hex", err)
	for i, w := range writtenPages {
		if !w {
			log.Fatalf("PROGRAM ERROR: Did not write full memory! Missing page %d", i)
		}
	}
	trimmed := arduboy.TrimUnused(sketch, arduboy.FlashPageSize)
	log.Printf("Wrote %d bytes to %s (sketch was %d)\n", len(sketch), d.SmallString(), len(trimmed))
	if c.Runnow {
		arduboy.ExitBootloader(sercon)
	}
	fullhash := arduboy.Md5String(sketch)
	hash := arduboy.Md5String(trimmed)
	// Return data about the eeprom (does this even matter?)
	result := make(map[string]interface{})
	result["Filename"] = c.Infile
	result["SketchLength"] = len(trimmed)
	result["SketchMD5"] = hash
	result["UsableFlashLength"] = len(sketch)
	result["UsableFlashMD5"] = fullhash
	PrintJson(result)
	return nil
}

// **********************************
// *       EEPROM COMMANDS          *
// **********************************

// Eeprom read command
type EepromReadCmd struct {
	Device  string `arg:"" help:"The system device to read from (use 'any' for first)"`
	Outfile string `type:"path" short:"o"`
}

func (c *EepromReadCmd) Run() error {
	// Figure out save location
	if c.Outfile == "" {
		c.Outfile = fmt.Sprintf("eeprom_%s.bin", FileSafeDateTime())
	}
	// Read eeprom
	sercon, d := connectWithBootloader(c.Device)
	defer sercon.Close()
	eeprom, err := arduboy.ReadEeprom(sercon)
	fatalIfErr(c.Device, "read eeprom", err)
	log.Printf("Read %d bytes from %s (full eeprom)\n", len(eeprom), d.SmallString())
	hash := arduboy.Md5String(eeprom)
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
	Infile string `type:"existingfile" short:"i"`
}

func (c *EepromWriteCmd) Run() error {
	sercon, d := connectWithBootloader(c.Device)
	defer sercon.Close()
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
	defer sercon.Close()
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
	Device string `arg:"" help:"The system device OR file to read from (use 'any' for first device)"`
	Html   bool   `help:"Generate as html instead"`
	Images bool   `help:"Pull images (takes 4 times as long)"`
}

func (c *FlashcartScanCmd) Run() error {
	var result []arduboy.HeaderCategory
	deviceIsFile := false
	deviceId := "" // Some identifiers computed based on file vs device
	deviceName := ""
	fileInfo, err := os.Stat(c.Device)
	deviceIsFile = (err == nil && fileInfo.Mode().IsRegular())
	// Can scan either flashcart file or the real device
	if deviceIsFile {
		log.Printf("%s is a file, scanning file\n", c.Device)
		data, err := os.Open(c.Device)
		fatalIfErr(c.Device, "open flashcart file", err)
		deviceId = c.Device
		deviceName = c.Device
		result, err = arduboy.ScanFlashcartFileMeta(data, c.Images)
		fatalIfErr(c.Device, "scan flashcart (file)", err)
	} else {
		sercon, d := connectWithBootloader(c.Device)
		extd := mustHaveFlashcart(sercon, d)
		deviceId = extd.Bootloader.Device
		deviceName = d.SmallString()
		result, err = arduboy.ScanFlashcartMeta(sercon, c.Images)
		fatalIfErr(c.Device, "scan flashcart (device)", err)
	}
	programs := 0
	for _, c := range result {
		programs += len(c.Slots)
	}
	log.Printf("Scanned %d categories, %d programs from flashcart on %s\n", len(result), programs, deviceName)
	if c.Html {
		err = arduboy.RenderFlashcartMeta(result, deviceId, os.Stdout)
		fatalIfErr(c.Device, "render flashcart into HTML", err)
	} else {
		PrintJson(result)
	}
	return nil
}

// Eeprom read command
type FlashcartReadCmd struct {
	Device  string `arg:"" help:"The system device to read from (use 'any' for first)"`
	Outfile string `type:"path" short:"o"`
}

func (c *FlashcartReadCmd) Run() error {
	// Figure out save location
	if c.Outfile == "" {
		c.Outfile = fmt.Sprintf("flashcart_%s.bin", FileSafeDateTime())
	}
	file, err := os.Create(c.Outfile)
	fatalIfErr(c.Outfile, "open file for writing", err)
	defer file.Close()
	// Read flashcart
	sercon, d := connectWithBootloader(c.Device)
	defer sercon.Close()
	_ = mustHaveFlashcart(sercon, d)
	length, slots, err := arduboy.ReadWholeFlashcart(sercon, file, true)
	fatalIfErr(c.Device, "read flashcart", err)
	log.Printf("Read %d bytes, %d slots from %s, wrote to %s\n", length, slots, d.SmallString(), c.Outfile)
	// Return data about the save
	result := make(map[string]interface{})
	result["Filename"] = c.Outfile
	result["Length"] = length
	result["Slots"] = slots
	PrintJson(result)
	return nil
}

// **********************************
// *       CONVERT COMMANDS         *
// **********************************

type Hex2BinCmd struct {
	Outfile string `type:"path" short:"o"`
	Infile  string `type:"existingfile" short:"i"`
}

func (c *Hex2BinCmd) Run() error {
	if c.Infile == "" {
		c.Infile = "sketch.hex"
	}
	if c.Outfile == "" {
		c.Outfile = fmt.Sprintf("sketch_hex2bin_%s.bin", FileSafeDateTime())
	}
	sketch, err := os.Open(c.Infile)
	fatalIfErr("hex2bin", "read hex file", err)
	defer sketch.Close()
	bin, err := arduboy.HexToBin(sketch)
	fatalIfErr("hex2bin", "convert hex", err)
	dest, err := os.Create(c.Outfile)
	fatalIfErr("hex2bin", "write file", err)
	defer dest.Close()
	dest.Write(bin)
	result := make(map[string]interface{})
	result["Infile"] = c.Infile
	result["Outfile"] = c.Outfile
	result["Bytes"] = len(bin)
	result["MD5"] = arduboy.Md5String(bin)
	PrintJson(result)
	return nil
}

type Bin2HexCmd struct {
	Outfile string `type:"path" short:"o"`
	Infile  string `type:"existingfile" short:"i"`
}

func (c *Bin2HexCmd) Run() error {
	if c.Infile == "" {
		c.Infile = "sketch.bin"
	}
	if c.Outfile == "" {
		c.Outfile = fmt.Sprintf("sketch_bin2hex_%s.hex", FileSafeDateTime())
	}
	sketch, err := os.ReadFile(c.Infile)
	fatalIfErr("bin2hex", "read bin file", err)
	dest, err := os.Create(c.Outfile)
	fatalIfErr("bin2hex", "write file", err)
	defer dest.Close()
	err = arduboy.BinToHex(sketch, dest)
	fatalIfErr("bin2hex", "convert bin", err)
	result := make(map[string]interface{})
	result["Infile"] = c.Infile
	result["Outfile"] = c.Outfile
	result["Bytes"] = len(sketch)
	result["MD5"] = arduboy.Md5String(sketch)
	PrintJson(result)
	return nil
}

// **********************************
// *    ALL TOGETHER COMMANDS       *
// **********************************

var cli struct {
	Scan struct {
		Devices   ScanCmd          `cmd:"" help:"Search for Arduboys and return basic information on them"`
		Flashcart FlashcartScanCmd `cmd:"" help:"Scan flashcart and return categories/games"`
	} `cmd:"" help:"Get cursory information on various things (devices, flashcart, etc)"`
	Analyze struct {
		Device QueryCmd `cmd:"" help:"Get deeper information about a particular Arduboy"`
	} `cmd:"" help:"Get deeper information on various things (device, flashcart, etc)"`
	Read struct {
		Sketch    SketchReadCmd    `cmd:"" help:"Read just the sketch portion of flash, saved as a .hex file"`
		Eeprom    EepromReadCmd    `cmd:"" help:"Read entire eeprom, saved as a .bin file"`
		Flashcart FlashcartReadCmd `cmd:"" help:"Read entire flashcart, saved as a .bin file"`
	} `cmd:"" help:"Read data from arduboy (sketch/flashcart/eeprom)"`
	Write struct {
		Eeprom EepromWriteCmd `cmd:"" help:"Write data to eeprom"`
		Rawhex RawHexWriteCmd `cmd:"" help:"Write hex file to arduboy precisely as-is"`
		Sketch SketchWriteCmd `cmd:"" help:"Write arduboy hex file to arduboy (standard procedure)"`
	} `cmd:"" help:"Write data to arduboy (sketch/flashcart/eeprom)"`
	Delete struct {
		Eeprom EepromDeleteCmd `cmd:"" help:"Reset entire eeprom"`
	} `cmd:"" help:"Delete data on arduboy (eeprom)"`
	Convert struct {
		Hex2Bin Hex2BinCmd `cmd:"" help:"Convert hex to bin" name:"hex2bin"`
		Bin2Hex Bin2HexCmd `cmd:"" help:"Convert bin to hex" name:"bin2hex"`
	} `cmd:"" help:"Convert data formats back and forth (usually all on filesystem)"`
	Version kong.VersionFlag `help:"Show version information"`
	Norgb   bool             `help:"Disable all rgb while accessing device"`
}

func main() {
	ctx := kong.Parse(&cli,
		kong.Name("ardugotools"),
		kong.ShortUsageOnError(),
		kong.Description("A set of tools for working with Arduboy"),
		kong.Vars{
			"version": "0.1.0",
		},
	)
	if cli.Norgb {
		arduboy.SetRgbEnabledGlobal(false)
	}
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
