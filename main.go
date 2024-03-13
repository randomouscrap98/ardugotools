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
	return sercon, d
}

func mustHaveFlashcart(sercon io.ReadWriteCloser, device *arduboy.BasicDeviceInfo) {
	extdata, err := arduboy.QueryDevice(device, sercon, false)
	fatalIfErr(device.Port, "check for flashcart", err)
	log.Printf("Flashcart: %v", extdata.Jedec)
	if extdata.Jedec == nil {
		log.Fatalf("Device %s doesn't seem to have a flashcart!", extdata.Bootloader.Device)
	}
}

// Scan command
type ScanCmd struct {
}

func (c *ScanCmd) Run() error {
	devices, err := arduboy.GetBasicDevices()
	fatalIfErr("scan", "pull devices", err)
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
	PrintJson(extdata)
	return nil
}

// Sketch read command
type SketchReadCmd struct {
	Device  string `arg:"" help:"The system device to read from (use 'any' for first)"`
	Outfile string `short:"o"`
}

func (c *SketchReadCmd) Run() error {
	// Read sketch First
	sercon, _ := connectWithBootloader(c.Device)
	sketch, err := arduboy.ReadSketch(sercon)
	fatalIfErr(c.Device, "read sketch", err)
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
	// Return data about the save
	result := make(map[string]interface{})
	result["Filename"] = c.Outfile
	result["MD5"] = hash
	PrintJson(result)
	return nil
}

// Flashcart scan command
type FlashcartScanCmd struct {
	Device string `arg:"" help:"The system device to read from (use 'any' for first)"`
	Html   bool   `help:"Generate as html instead"`
	Images bool   `help:"Pull images (takes 4 times as long)"`
}

func (c *FlashcartScanCmd) Run() error {
	sercon, d := connectWithBootloader(c.Device)
	mustHaveFlashcart(sercon, d)
	result, err := arduboy.ScanFlashcartMeta(sercon, c.Images)
	fatalIfErr(c.Device, "scan flashcart (basic)", err)
	PrintJson(result)
	return nil
}

var cli struct {
	Scan   ScanCmd  `cmd:"" help:"Search for Arduboys and return basic information on them"`
	Query  QueryCmd `cmd:"" help:"Get deeper information about a particular Arduboy"`
	Sketch struct {
		Read SketchReadCmd `cmd:"" help:"Read just the sketch portion of flash, saved as a .hex file"`
	} `cmd:"" help:"Perform actions on the builtin flash or related to sketch files"`
	Flashcart struct {
		Scan FlashcartScanCmd `cmd:"" help:"Scan flashcart and return categories/games"`
	} `cmd:"" help:"Perform actions on the 'external' flashcart (FX/Mini/etc)"`
}

func main() {
	ctx := kong.Parse(&cli)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
