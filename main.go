package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"time"

	"github.com/alecthomas/kong"
	"github.com/mazznoer/csscolorparser"
	"github.com/randomouscrap98/ardugotools/arduboy"
)

const (
	AppVersion = "0.3.0"
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
	Device string `arg:"" default:"any" help:"The system device to check (use 'any' for first)"`
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
	Device  string `arg:"" default:"any" help:"The system device to read from (use 'any' for first)"`
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
	Device string `arg:"" default:"any" help:"The system device to write to (use 'any' for first)"`
	Infile string `type:"existingfile" default:"sketch.hex" short:"i" help:"File to load hex from"`
	Runnow bool   `help:"Run sketch immediately"`
}

func (c *RawHexWriteCmd) Run() error {
	sercon, d := connectWithBootloader(c.Device)
	defer sercon.Close()
	// Go find the file first
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
	Device string `arg:"" default:"any" help:"The system device to write to (use 'any' for first)"`
	Infile string `type:"existingfile" default:"sketch.hex" short:"i" help:"File to load hex from"`
	Runnow bool   `help:"Run sketch immediately"`
}

func (c *SketchWriteCmd) Run() error {
	sercon, d := connectWithBootloader(c.Device)
	defer sercon.Close()
	// Go find the file first
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
	Device  string `arg:"" default:"any" help:"The system device to read from (use 'any' for first)"`
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
	Device string `arg:"" default:"any" help:"The system device to read from (use 'any' for first)"`
	Infile string `type:"existingfile" default:"eeprom.bin" short:"i"`
}

func (c *EepromWriteCmd) Run() error {
	sercon, d := connectWithBootloader(c.Device)
	defer sercon.Close()
	// Go find the file first
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
	Device string `arg:"" default:"any" help:"The system device to read from (use 'any' for first)"`
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
	Device string `arg:"" default:"any" help:"The system device OR file to read from (use 'any' for first device)"`
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

// Flashcart read command
type FlashcartReadCmd struct {
	Device  string `arg:"" default:"any" help:"The system device to read from (use 'any' for first)"`
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

// Flashcart write command
type FlashcartWriteCmd struct {
	Device           string `arg:"" default:"any" help:"The system device to write to (use 'any' for first)"`
	Infile           string `type:"existingfile" default:"flashcart.bin" short:"i"`
	OverrideCapacity int    `help:"Force device capacity (NOT RECOMMENDED)"`
	Noverify         bool   `help:"Do not verify flashcart (not recommended)"`
}

func (c *FlashcartWriteCmd) Run() error {
	// Open arduboy, force flashcart existence
	sercon, d := connectWithBootloader(c.Device)
	defer sercon.Close()
	extdata := mustHaveFlashcart(sercon, d)
	if c.OverrideCapacity > 0 {
		// Spooky user desires
		extdata.Jedec.Capacity = int32(c.OverrideCapacity)
	}
	// Figure out save location, open file
	file, err := os.Open(c.Infile)
	fatalIfErr(c.Infile, "open file for reading", err)
	defer file.Close()
	fi, err := file.Stat()
	fatalIfErr(c.Infile, "check file", err)
	fileSize := int32(fi.Size())
	if !extdata.Jedec.FitsFlashcart(fileSize) {
		log.Fatalf("Flashcart too big for device! Size: %d, capacity: %d\n",
			fileSize, extdata.Jedec.Capacity)
	}
	// Actually write the thing
	blocks, err := arduboy.WriteWholeFlashcart(sercon, file, !c.Noverify, true)
	fatalIfErr(c.Device, "write flashcart", err)
	log.Printf("Finished writing %d blocks to flashcart (%d bytes)\n",
		blocks, blocks*arduboy.FXBlockSize)
	// Return data about the save
	result := make(map[string]interface{})
	result["Filename"] = c.Infile
	result["Length"] = fileSize
	result["Written"] = blocks * arduboy.FXBlockSize
	result["Capacity"] = extdata.Jedec.Capacity
	result["Verified"] = !c.Noverify
	PrintJson(result)
	return nil
}

// **********************************
// *       CONVERT COMMANDS         *
// **********************************

// ------------ Sketches --------------
type Hex2BinCmd struct {
	Outfile string `type:"path" short:"o"`
	Infile  string `type:"existingfile" default:"sketch.hex" short:"i"`
}

func (c *Hex2BinCmd) Run() error {
	if c.Outfile == "" {
		c.Outfile = fmt.Sprintf("sketch_hex2bin_%s.bin", FileSafeDateTime())
	}
	sketch, err := os.Open(c.Infile)
	fatalIfErr("hex2bin", "read hex file", err)
	defer sketch.Close()
	bin, err := arduboy.HexToBin(sketch)
	fatalIfErr("hex2bin", "convert hex", err)
	log.Printf("Hex real data length is %d\n", len(bin))
	dest, err := os.Create(c.Outfile)
	fatalIfErr("hex2bin", "write file", err)
	defer dest.Close()
	dest.Write(bin)
	result := make(map[string]interface{})
	result["Infile"] = c.Infile
	result["Outfile"] = c.Outfile
	result["Length"] = len(bin)
	result["MD5"] = arduboy.Md5String(bin)
	PrintJson(result)
	return nil
}

type Bin2HexCmd struct {
	Outfile string `type:"path" short:"o"`
	Infile  string `type:"existingfile" default:"sketch.bin" short:"i"`
}

func (c *Bin2HexCmd) Run() error {
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
	result["Length"] = len(sketch)
	result["MD5"] = arduboy.Md5String(sketch)
	PrintJson(result)
	return nil
}

// ----------------- Images -------------------
type Img2BinCmd struct {
	Outfile   string `type:"path" short:"o"`
	Infile    string `default:"image.png" type:"existingfile" short:"i"`
	Threshold uint8  `default:"100" help:"White threshold (grayscale value)"`
}

func (c *Img2BinCmd) Run() error {
	if c.Outfile == "" {
		c.Outfile = fmt.Sprintf("image_img2bin_%s.bin", FileSafeDateTime())
	}
	img, err := os.Open(c.Infile)
	fatalIfErr("img2bin", "read image file", err)
	defer img.Close()
	stat, err := img.Stat()
	fatalIfErr("img2bin", "getFileInfo", err)
	paletted, err := arduboy.RawImageToPalettedTitle(img, c.Threshold)
	fatalIfErr("img2bin", "convert image to palette", err)
	bin, err := arduboy.PalettedToRawTitle(paletted)
	fatalIfErr("img2bin", "convert palette to raw", err)
	err = os.WriteFile(c.Outfile, bin, 0644)
	fatalIfErr("img2bin", "write file", err)
	result := make(map[string]interface{})
	result["Infile"] = c.Infile
	result["Outfile"] = c.Outfile
	result["BinLength"] = len(bin)
	result["ImageLength"] = stat.Size()
	result["MD5"] = arduboy.Md5String(bin)
	PrintJson(result)
	return nil
}

type Bin2ImgCmd struct {
	Outfile string `type:"path" short:"o"`
	Infile  string `type:"existingfile" default:"image.bin" short:"i"`
	Format  string `enum:"png,gif,bmp,jpg" default:"png" help:"Image output format"`
	Black   string `default:"#000000" help:"Color to use for black"`
	White   string `default:"#FFFFFF" help:"Color to use for white"`
}

func (c *Bin2ImgCmd) Run() error {
	if c.Outfile == "" {
		c.Outfile = fmt.Sprintf("image_bin2img_%s.%s", FileSafeDateTime(), c.Format)
	}
	raw, err := os.ReadFile(c.Infile)
	fatalIfErr("bin2img", "read bin file", err)
	paletted, err := arduboy.RawToPalettedTitle(raw)
	fatalIfErr("bin2img", "convert to paletted", err)
	black, err := csscolorparser.Parse(c.Black)
	fatalIfErr("bin2img", "parse black color", err)
	white, err := csscolorparser.Parse(c.White)
	fatalIfErr("bin2img", "parse white color", err)
	imgfile, err := os.Create(c.Outfile)
	fatalIfErr("bin2img", "create file", err)
	defer imgfile.Close()
	err = arduboy.PalettedToImage(paletted, arduboy.ScreenWidth, arduboy.ScreenHeight,
		black, white, c.Format, imgfile)
	fatalIfErr("bin2img", "convert paletted to "+c.Format, err)
	stat, err := imgfile.Stat()
	fatalIfErr("bin2img", "get image file info", err)
	result := make(map[string]interface{})
	result["Infile"] = c.Infile
	result["Outfile"] = c.Outfile
	result["BinLength"] = len(raw)
	result["ImageLength"] = stat.Size()
	result["MD5"] = arduboy.Md5String(raw)
	PrintJson(result)
	return nil
}

type Img2ImgCmd struct {
	Outfile   string `type:"path" short:"o"`
	Infile    string `type:"existingfile" default:"image.png" short:"i"`
	Format    string `enum:"png,gif,bmp,jpg" default:"png" help:"Image output format"`
	Black     string `default:"#000000" help:"Color to use for black"`
	White     string `default:"#FFFFFF" help:"Color to use for white"`
	Threshold uint8  `default:"100" help:"White threshold (grayscale value)"`
}

func (c *Img2ImgCmd) Run() error {
	if c.Outfile == "" {
		c.Outfile = fmt.Sprintf("image_convert_%s.%s", FileSafeDateTime(), c.Format)
	}
	original, err := os.Open(c.Infile)
	fatalIfErr("img2img", "read image file", err)
	defer original.Close()
	stat, err := original.Stat()
	fatalIfErr("img2bin", "getFileInfo", err)
	paletted, err := arduboy.RawImageToPalettedTitle(original, c.Threshold)
	fatalIfErr("img2img", "convert to paletted", err)
	black, err := csscolorparser.Parse(c.Black)
	fatalIfErr("img2img", "parse black color", err)
	white, err := csscolorparser.Parse(c.White)
	fatalIfErr("img2img", "parse white color", err)
	imgfile, err := os.Create(c.Outfile)
	fatalIfErr("img2img", "write file", err)
	defer imgfile.Close()
	err = arduboy.PalettedToImage(paletted, arduboy.ScreenWidth, arduboy.ScreenHeight,
		black, white, c.Format, imgfile)
	fatalIfErr("img2img", "convert paletted to "+c.Format, err)
	newstat, err := imgfile.Stat()
	fatalIfErr("img2img", "get new file stat", err)
	result := make(map[string]interface{})
	result["Infile"] = c.Infile
	result["Outfile"] = c.Outfile
	result["InputImageLength"] = stat.Size()
	result["OutputImageLength"] = newstat.Size()
	PrintJson(result)
	return nil
}

type SplitCodeCmd struct {
	Config         arduboy.TileConfig `embed:""`
	Gentiles       string             `type:"path" short:"t"`
	Black          string             `default:"#000000" help:"Color to use for black for gentiles"`
	White          string             `default:"#FFFFFF" help:"Color to use for white for gentiles"`
	Threshold      uint8              `default:"100" help:"White threshold (grayscale value)"`
	Alphathreshold uint8              `default:"50" help:"Alpha threshold (values lower are 'transparent')"`
	Infile         string             `type:"existingfile" default:"spritesheet.png" short:"i"`
	Nocomments     bool               `help:"Don't generate the comments at the top of code"`
}

func (c *SplitCodeCmd) Run() error {
	log.Printf("Config: %v\n", c.Config)
	sprites, err := os.Open(c.Infile)
	fatalIfErr("splitcode", "read image file", err)
	defer sprites.Close()
	stat, err := sprites.Stat()
	fatalIfErr("splitcode", "get file info", err)
	tiles, computed, err := arduboy.SplitImageToTiles(sprites, &c.Config)
	fatalIfErr("splitcode", "split image to tiles", err)
	log.Printf("Split into %d %dx%d tiles\n", len(tiles), computed.SpriteWidth, computed.SpriteHeight)
	// Maybe too much memory? IDK
	ptiles := make([][]byte, len(tiles))
	for i, tile := range tiles {
		ptiles[i], _, _ = arduboy.ImageToPaletted(tile, c.Threshold, c.Alphathreshold)
	}
	if c.Gentiles != "" {
		// Go try to make the folder
		err = os.Mkdir(c.Gentiles, 0770)
		fatalIfErr("splitcode", "create tiles folder", err)
		black, err := csscolorparser.Parse(c.Black)
		fatalIfErr("splitcode", "parse black color", err)
		white, err := csscolorparser.Parse(c.White)
		fatalIfErr("splictcode", "parse white color", err)
		// Now for each image, dump it as a png
		for i, ptile := range ptiles {
			tpath := path.Join(c.Gentiles, fmt.Sprintf("%d.png", i))
			tfile, err := os.Create(tpath)
			fatalIfErr("splitcode", fmt.Sprintf("write tile %d", i), err)
			log.Printf("Writing tile file %s\n", tpath)
			arduboy.PalettedToImage(ptile, computed.SpriteWidth,
				computed.SpriteHeight, black, white, "png", tfile)
		}
	}

	if !c.Nocomments {
		fmt.Printf("// Generated on %s with ardugotools %s\n", time.Now().Format(time.RFC1123), AppVersion)
		fmt.Printf("// Original file: %s (%d bytes)\n", path.Base(c.Infile), stat.Size())
		fmt.Printf("// Tilesize: %dx%d Spacing: %d\n",
			computed.SpriteWidth, computed.SpriteHeight, c.Config.Spacing)
		fmt.Printf("\n")
	}

	// Now generate the actual code
	code, err := arduboy.PalettedToCode(ptiles, &c.Config, computed)
	fmt.Printf(code)

	return nil
}

// **********************************
// *    ALL TOGETHER COMMANDS       *
// **********************************

var cli struct {
	Device struct {
		Scan  ScanCmd  `cmd:"" help:"Search for Arduboys and return basic information on them"`
		Query QueryCmd `cmd:"" help:"Get deeper information about a particular Arduboy"`
	} `cmd:"" help:"Commands which retrieve information about devices"`
	Sketch struct {
		Read     SketchReadCmd  `cmd:"" help:"Read just the sketch portion of flash, saved as a .hex file"`
		Write    SketchWriteCmd `cmd:"" help:"Write arduboy hex file to arduboy (standard procedure)"`
		WriteRaw RawHexWriteCmd `cmd:"" help:"Write hex file to arduboy precisely as-is"`
		Hex2Bin  Hex2BinCmd     `cmd:"" help:"Convert sketch hex to bin" name:"hex2bin"`
		Bin2Hex  Bin2HexCmd     `cmd:"" help:"Convert sketch bin to hex" name:"bin2hex"`
		// Could analyze sketch to figure out what device it might be for
	} `cmd:"" help:"Commands which work directly on sketches, whether on device or filesystem"`
	Eeprom struct {
		Read   EepromReadCmd   `cmd:"" help:"Read entire eeprom, saved as a .bin file"`
		Write  EepromWriteCmd  `cmd:"" help:"Write data to eeprom"`
		Delete EepromDeleteCmd `cmd:"" help:"Reset entire eeprom"`
	} `cmd:"" help:"Commands which work directly on eeprom, whether on device or filesystem"`
	Flashcart struct {
		Scan  FlashcartScanCmd  `cmd:"" help:"Scan flashcart and return categories/games (works on files too)"`
		Read  FlashcartReadCmd  `cmd:"" help:"Read entire flashcart, saved as a .bin file"`
		Write FlashcartWriteCmd `cmd:"" help:"Write full flashcart to arduboy"`
		// Could analyze flashcart to figure out what device it might be for, and whether
		// it's technically invalid
	} `cmd:"" help:"Commands which work directly on flashcarts, whether on device or filesystem"`
	Image struct {
		Bin2Img   Bin2ImgCmd   `cmd:"" help:"Convert 1024 byte bin to png img" name:"bin2img"`
		Img2Bin   Img2BinCmd   `cmd:"" help:"Convert any image to arduboy 1024 byte bin format" name:"img2bin"`
		Img2Title Img2ImgCmd   `cmd:"" help:"Convert any image to a 2 color 128x64 black and white image" name:"img2title"`
		SplitCode SplitCodeCmd `cmd:"" help:"Split image, generate code" name:"splitcode"`
	} `cmd:"" help:"Commands which work directly on images, such as titles or spritesheets"`
	Version kong.VersionFlag `help:"Show version information"`
	Norgb   bool             `help:"Disable all rgb while accessing device"`
}

func main() {
	ctx := kong.Parse(&cli,
		kong.Name("ardugotools"),
		kong.ShortUsageOnError(),
		kong.Description("A set of tools for working with Arduboy"),
		kong.Vars{
			"version": AppVersion,
		},
	)
	if cli.Norgb {
		arduboy.SetRgbEnabledGlobal(false)
	}
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
