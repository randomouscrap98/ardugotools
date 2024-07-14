package arduboy

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type FlashcartReader struct {
	File *os.File
	//Address int // Current address within the flashcart
}

type FlashcartWriter struct {
	File         *os.File
	CategoryId   int
	Slots        int
	LastSlotPage uint16
	//Address int // Current address within the flashcart
	// TODO: add settings for menu, contrast, screen patching, etc
	ValidateCategoryStructure bool
	ValidateImageLength       bool
	PatchMenu                 bool
	PatchMicroLED             bool
	PatchSsd1309              bool
	Contrast                  int
}

func NewFlashcartWriter(file *os.File) *FlashcartWriter {
	return &FlashcartWriter{
		File:                      file,
		CategoryId:                -1,     //Start at -1 to make incrementing for categories easier
		LastSlotPage:              0xFFFF, // first slot always has this as 0xFFFF
		ValidateCategoryStructure: true,
		ValidateImageLength:       true,
		PatchMenu:                 true,
		PatchMicroLED:             false,
		PatchSsd1309:              false,
		Contrast:                  CONTRAST_NOCHANGE,
	}
}

// General tracking for entire lua script (user can open arbitrary flashcarts)
type FlashcartState struct {
	FileDirectory string
	Readers       []*FlashcartReader
	Writers       []*FlashcartWriter
	Arguments     []string
}

// Get full path to given file requested by user. The system has a way to set
// the "working directory" for the whole script, that's all
func (state *FlashcartState) FilePath(path string) string {
	if state.FileDirectory == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(state.FileDirectory, path)
}

// Add a function to the given lua state that actually tracks with our own state.
// Usually lua functions don't accept extra go parameters
func (state *FlashcartState) AddFunction(name string, f func(*lua.LState, *FlashcartState) int, L *lua.LState) {
	L.SetGlobal(name, L.NewFunction(func(L *lua.LState) int { return f(L, state) }))
}

// Attempt to close all open files
func (state *FlashcartState) CloseAll() []error {
	results := make([]error, 0)
	for _, f := range state.Readers {
		err := f.File.Close()
		if err != nil {
			log.Printf("ERROR: Couldn't close pending reader: %s", err)
			results = append(results, err)
		}
	}
	for _, f := range state.Writers {
		log.Printf("Closing flashcart writer '%s'", f.File.Name())
		// Before closing, you need to write the final 1 page of 0xFF
		_, err := f.File.Write(MakePadding(FXPageSize))
		if err != nil {
			log.Printf("ERROR: Couldn't write final page of padding: %s", err)
			results = append(results, err)
		}
		err = f.File.Close()
		if err != nil {
			log.Printf("ERROR: Couldn't close pending writer: %s", err)
			results = append(results, err)
		}
	}
	return results
}

func (writer *FlashcartWriter) initHeader() FxHeader {
	return FxHeader{
		PreviousPage: writer.LastSlotPage,
		NextPage:     0xFFFF,
		ProgramStart: 0xFFFF,
		DataStart:    0xFFFF,
		DataPages:    0xFFFF, // This is how old programs did it
		SaveStart:    0xFFFF,
	}
}

// Write the entirety of a slot given as a table as the first param. Should
// have some expected fields; most the header stuff is calculated in this
// function though.
func (writer *FlashcartWriter) WriteSlot(L *lua.LState) int {
	slot := L.ToTable(1)
	if slot == nil {
		L.RaiseError("Must send slot to write_slot!")
		return 0
	}
	addr, err := writer.File.Seek(0, io.SeekCurrent)
	if err != nil {
		L.RaiseError("Couldn't determine current seek position on slot %d: %s", writer.Slots, err)
		return 0
	}
	// Addr is now the beginning of the slot. Any other calcs should be
	// based off this value
	header := writer.initHeader()
	var slotSize = FXPageSize + FxHeaderImageLength
	slotEnd := func() uint16 {
		return uint16((int(addr) + slotSize) / FXPageSize)
	}
	var image, sketch, fxdata, fxsave []byte
	//pullBool(slot, "is_category", func(is bool) { is_category = is })
	pullString(slot, "title", func(t string) { header.Title = t })
	pullString(slot, "version", func(v string) { header.Version = v })
	pullString(slot, "developer", func(d string) { header.Developer = d })
	pullString(slot, "info", func(i string) { header.Info = i })
	pullString(slot, "image", func(i string) { image = []byte(i) })
	pullString(slot, "sketch", func(s string) { sketch = []byte(s) })
	pullString(slot, "fxdata", func(d string) { fxdata = []byte(d) })
	pullString(slot, "fxsave", func(s string) { fxsave = []byte(s) })
	//log.Printf("Write initial lengths: s:%d, fd:%d, fs:%d", len(sketch), len(fxdata), len(fxsave))
	is_category := len(sketch) == 0
	if len(image) != FxHeaderImageLength {
		if writer.ValidateImageLength {
			L.RaiseError("Invalid image length on slot %d!", writer.Slots)
			return 0
		} else if len(image) < FxHeaderImageLength {
			image = AlignData(image, FxHeaderImageLength)
		} else {
			image = image[:FxHeaderImageLength]
		}
	}
	if is_category {
		writer.CategoryId += 1
	} else {
		if writer.Slots < 2 && writer.ValidateCategoryStructure {
			L.RaiseError("First two slots MUST be categories! ")
			return 0
		}
		// Wasteful but it's like 32KiB max... we need the pre-modded sketch to calculate the sha256
		premodsketch := make([]byte, len(sketch), FlashSize)
		copy(premodsketch, sketch)
		premodsketch = AlignData(premodsketch, FXPageSize)
		// Pre-align all the data (only if not a category)
		if len(sketch) > 0 {
			if writer.PatchMenu {
				// TODO: patch menu if user asks
				patched, message := PatchMenuButtons(sketch)
				if patched {
					log.Printf(message)
				}
			}
			if writer.PatchMicroLED {
				PatchMicroLED(sketch)
			}
			patchcount := PatchScreen(sketch, writer.PatchSsd1309, writer.Contrast)
			if patchcount > 0 {
				log.Printf("Patched %d screen parameter(s) (ssd1309: %t, contrast: %x)", patchcount, writer.PatchSsd1309, writer.Contrast)
			}
			sketch = AlignData(sketch, FlashPageSize)
			pages := len(sketch) / FlashPageSize
			if pages > 0xFF {
				// Don't even consider the bootloader, there is a max size for the header
				L.RaiseError("Sketch in slot %d too large!", writer.Slots)
				return 0
			}
			header.ProgramStart = slotEnd()
			header.ProgramPages = uint8(pages) // length is PRE fx-padding...
			sketch = AlignData(sketch, FXPageSize)
			slotSize += len(sketch)
		}
		if len(fxdata) > 0 {
			if len(sketch) == 0 {
				L.RaiseError("FX data without sketch in slot %d!", writer.Slots)
				return 0
			}
			fxdata = AlignData(fxdata, FXPageSize)
			header.DataStart = slotEnd()
			header.DataPages = uint16(len(fxdata) / FXPageSize)
			slotSize += len(fxdata)
			// MUST patch the sketch to point to this data
			sketch[0x14] = 0x18
			sketch[0x15] = 0x95
			Write2ByteValue(header.DataStart, sketch, 0x16)
		}
		if len(fxsave) > 0 {
			if len(sketch) == 0 {
				L.RaiseError("FX save without sketch in slot %d!", writer.Slots)
				return 0
			}
			fxsave = AlignData(fxsave, FxSaveAlignment)
			// Need to align fx save to a 4K boundary. The alignment goes at the
			// BEGINNING of the save
			var prealignment int = int(AlignWidth(uint(addr)+uint(slotSize), FxSaveAlignment)) - int(addr) - int(slotSize)
			fxsave = append(MakePadding(prealignment), fxsave...)
			slotSize += prealignment
			header.SaveStart = slotEnd()
			slotSize += len(fxsave) - prealignment
			// MUST patch the sketch to point to this save
			sketch[0x18] = 0x18
			sketch[0x19] = 0x95
			Write2ByteValue(header.SaveStart, sketch, 0x1a)
		}
		// ONLY calculate hash if not a category (this is how old tools did it; it doesn't matter much)
		header.Sha256, err = calculateHeaderHash(premodsketch, fxdata)
		if err != nil {
			L.RaiseError("Couldn't hash header: %s", err)
			return 0
		}
	}
	// ALWAYS write the category (it tells which programs are in which category)
	header.Category = uint8(writer.CategoryId)
	// Finish up writing header values now that we know all alignments
	if slotSize&0xFF > 0 {
		L.RaiseError("ARDUGOTOOLS PROGRAM ERROR: Slot size misaligned: %d", slotSize)
		return 0
	}
	header.SlotPages = uint16(slotSize / FXPageSize)
	header.NextPage = slotEnd()
	// Create the header
	headerraw, err := header.MakeHeader()
	if err != nil {
		L.RaiseError("Couldn't compile header: %s", err)
		return 0
	}
	totalWritten := 0
	// Write out all the individual blocks of data
	sw := func(data []byte) bool {
		if len(data) > 0 {
			written, err := writer.File.Write(data)
			if err != nil {
				L.RaiseError("Couldn't write to flashcart: %s", err)
				return true
			}
			totalWritten += written
		}
		return false
	}
	//log.Printf("Write final lengths: h: %d, s:%d, fd:%d, fs:%d", len(headerraw), len(sketch), len(fxdata), len(fxsave))
	if sw(headerraw) || sw(image) || sw(sketch) || sw(fxdata) || sw(fxsave) {
		return 0
	}
	if totalWritten != slotSize {
		L.RaiseError("ARDUGOTOOLS PROGRAM ERROR: Expected to write %d for '%s', actually wrote %d", slotSize, header.Title, totalWritten)
		return 0
	}
	log.Printf("Wrote slot %d: '%s' (%d bytes)\n", writer.Slots, header.Title, slotSize)
	writer.Slots += 1
	writer.LastSlotPage = uint16(int(addr) / FXPageSize)
	L.Push(lua.LNumber(slotSize))
	return 1
}

// -----------------------------
//          FUNCTIONS
// -----------------------------

func luaIsCategory(L *lua.LState) int {
	slot := L.ToTable(1)
	if slot == nil {
		L.RaiseError("Must send slot to is_category!")
		return 0
	}
	result := true
	pullString(slot, "sketch", func(s string) { result = (len(s) == 0) })
	// Just double check.
	// WARN: This could produce really strange errors, this is probably bad!
	// Specifically: if a user loads an existing category slot then adds an
	// empty sketch to it expecting it to turn it into a non-category. I suppose
	// that doens't really matter though?
	if result {
		// Remember: the function is only run if it was found!
		pullBool(slot, "was_category", func(b bool) { result = b })
	}
	L.Push(lua.LBool(result))
	return 1
}

func luaHasFxsave(L *lua.LState) int {
	slot := L.ToTable(1)
	if slot == nil {
		L.RaiseError("Must send slot to has_fxsave!")
		return 0
	}
	result := false
	pullString(slot, "fxsave", func(s string) { result = len(s) > 0 })
	L.Push(lua.LBool(result))
	return 1
}

func luaParseFlashcart(L *lua.LState, state *FlashcartState) int {
	relpath := L.ToString(1)
	preload := L.ToBool(2)
	filepath := state.FilePath(relpath)
	// Attempt to open the file first.
	file, err := os.Open(filepath)
	if err != nil {
		L.RaiseError("Error opening flashcart: %s", err)
		return 0
	}
	// Now that we have a working file, we must immediately add it to the
	// readers. The readers list is automatically cleaned up
	state.Readers = append(state.Readers, &FlashcartReader{
		File: file,
	})
	// WAS going to have an iterator, but functions aren't set up for that.
	// Simply parse out all the headers using the existing ScanFlashcartFile()
	var result lua.LTable
	scanFunc := func(f io.ReadSeeker, header *FxHeader, addr int, index int) error {
		if header.IsOldFormat() {
			// TODO: eventually, you will need to support this!
			return fmt.Errorf("Flashcart is in older format (no DataPages field set); can't parse")
		}
		var slot lua.LTable
		slot.RawSetString("title", lua.LString(header.Title))
		slot.RawSetString("version", lua.LString(header.Version))
		slot.RawSetString("developer", lua.LString(header.Developer))
		slot.RawSetString("info", lua.LString(header.Info))
		slot.RawSetString("was_category", lua.LBool(header.IsCategory()))
		simpleScan := func(field string, at int, length int) error {
			raw := make([]byte, length)
			if length != 0 {
				err := SeekRead(f, int64(at), raw)
				if err != nil {
					return err
				}
			}
			slot.RawSetString(field, lua.LString(string(raw)))
			return nil
		}
		// Read the image too (it is safe to seek the file any time)
		if err := simpleScan("image", addr+FXPageSize, FxHeaderImageLength); err != nil {
			return err
		}
		// Make up a pullData function, which will populate the data fields in
		// our slot.
		pullData := func() error {
			if header.IsCategory() {
				log.Printf("Tried to pull data for category (ignoring)")
				return nil
			}
			// Read the sketch, which is always doable
			if err := simpleScan("sketch", int(header.ProgramStart)*FXPageSize, int(header.ProgramPages)*FlashPageSize); err != nil {
				return err
			}
			// Try to read fx data and save, if they exist. But ALWAYS set the fields so
			// users aren't confused? I don't know...
			fxDataSize := 0
			fxSaveSize := 0
			dataStart := int(header.DataStart) * FXPageSize
			saveStart := int(header.SaveStart) * FXPageSize
			if header.HasFxData() {
				fxDataSize = int(header.DataPages) * FXPageSize
			}
			if header.HasFxSave() {
				fxSaveSize = addr + int(header.SlotPages)*FXPageSize - saveStart
			}
			if err := simpleScan("fxdata", dataStart, fxDataSize); err != nil {
				return err
			}
			if err := simpleScan("fxsave", saveStart, fxSaveSize); err != nil {
				return err
			}
			return nil
		}
		// Allow users to pull data from the file when needed
		slot.RawSetString("pull_data", L.NewFunction(func(IL *lua.LState) int {
			err := pullData()
			if err != nil {
				IL.RaiseError("Error pulling data from slot #%d at %x: %s", index, addr, err)
			}
			return 0
		}))
		// Assign the table to the right index in the result (1 based)
		result.RawSetInt(index+1, &slot)
		// Go ahead and pull in the data NOW if the user requested
		if preload {
			return pullData()
		}
		return nil
	}
	// Actually perform the scan. This will scan through the ENTIRE file, parsing out
	// the lua tables for every slot. The user will then have access to all the slots
	// immediately, in case they want to reorder or whatever.
	_, err = ScanFlashcartFile(file, scanFunc)
	if err != nil {
		L.RaiseError("Error scanning flashcart: %s", err)
		return 0
	}
	L.Push(&result)
	return 1
}

func luaNewFlashcart(L *lua.LState, state *FlashcartState) int {
	relpath := L.ToString(1)
	fp := state.FilePath(relpath)
	log.Printf("Opening new flashcart: %s", fp)
	// Attempt to create the file first.
	file, err := os.Create(fp)
	if err != nil {
		L.RaiseError("Error creating new flashcart: %s", err)
		return 0
	}
	// Now that we have a working file, we must immediately add it to the
	// writers. The writers list is automatically cleaned up
	writer := NewFlashcartWriter(file)
	state.Writers = append(state.Writers, writer)
	var result lua.LTable
	result.RawSetString("write_slot", L.NewFunction(func(IL *lua.LState) int {
		return writer.WriteSlot(IL)
	}))
	L.Push(&result)
	return 1
}

func luaGetArguments(L *lua.LState, state *FlashcartState) int {
	for _, arg := range state.Arguments {
		L.Push(lua.LString(arg))
	}
	return len(state.Arguments)
}

// Create the raw bytes for a converted title image
func luaTitleImage(L *lua.LState, state *FlashcartState) int {

	filename := L.ToString(1)
	threshold := L.ToInt(6) // The upper bound for black pixels

	// Validation and/or setting the defaults if not set
	if filename == "" {
		L.RaiseError("Must provide filename for image!")
		return 0
	}
	if threshold == 0 {
		threshold = 100
	}

	file, err := os.Open(state.FilePath(filename))
	if err != nil {
		L.RaiseError("Error opening image file: %s", err)
		return 0
	}
	defer file.Close()
	paletted, err := RawImageToPalettedTitle(file, uint8(threshold))
	if err != nil {
		L.RaiseError("Error converting image to title: %s", err)
		return 0
	}
	raw, _, err := PalettedToRaw(paletted, ScreenWidth, ScreenHeight)
	if err != nil {
		L.RaiseError("Can't convert title raw: %s", err)
		return 0
	}

	log.Printf("Converted image '%s' to title", filename)
	L.Push(lua.LString(string(raw))) // Actual raw data

	return 1
}

func luaPackageReader(L *lua.LState, state *FlashcartState, readAny bool) int {
	filename := L.ToString(1)
	device := L.ToString(2)

	var exact string
	var threshold int

	if readAny {
		threshold = L.ToInt(3)
	} else {
		exact = L.ToString(3)
		threshold = L.ToInt(4)
	}

	if threshold <= 0 {
		threshold = 100
	}

	realfilepath := state.FilePath(filename)

	archive, err := zip.OpenReader(realfilepath)
	if err != nil {
		L.RaiseError("Can't open arduboy archive: %s", err)
		return 0
	}
	defer archive.Close()

	info, err := ReadPackageInfo(archive)
	if err != nil {
		L.RaiseError("Couldn't read info.json in package '%s': %s", filename, err)
		return 0
	}
	if info.Title == "" {
		fname := filepath.Base(realfilepath)
		info.Title = strings.TrimSuffix(fname, filepath.Ext(fname))
		log.Printf("WARN: no title set in info.json, defaulting to %s", info.Title)
	}

	//log.Printf("PACKAGE INFO: %v", info)
	//log.Printf("PACKAGE[0] INFO: %v", info.Binaries[0])

	// With the info retrieved, figure out what the heck we need to get out of it.
	// If there are multiple options for what the user specified as a filter, we must
	// always quit with an error, because I don't want this tool picking for them.
	// Exact name always overrides device, so they can pass empty string for one/other
	var slot lua.LTable
	slot.RawSetString("title", lua.LString(info.Title))
	slot.RawSetString("info", lua.LString(info.Description))
	slot.RawSetString("developer", lua.LString(info.Author))
	slot.RawSetString("version", lua.LString(info.Version))

	var binary *PackageBinary

	if readAny {
		devices := strings.Split(device, ",")
		for i := range devices {
			devices[i] = strings.Trim(devices[i], " ")
		}
		binary, err = FindAnyBinary(&info, devices)
	} else {
		binary, err = FindSuitableBinary(&info, device, exact)
	}
	if err != nil {
		L.RaiseError("Error in package %s: %s", filename, err)
		return 0
	}

	// Load the easy stuff
	sketchreader, err := archive.Open(binary.Filename)
	if err != nil {
		L.RaiseError("Couldn't open sketch: %s", err)
		return 0
	}
	defer sketchreader.Close()
	sketch, err := HexToBin(sketchreader)
	if err != nil {
		L.RaiseError("Couldn't convert sketch: %s", err)
		return 0
	}
	slot.RawSetString("sketch", lua.LString(string(sketch)))
	log.Printf("Package %s sketch: %d bytes", realfilepath, len(sketch))

	// NOTE: WE DO NOT FIX BAD FX DATA! WE DO NOT STRIP THE SAVE OUT!
	if binary.FlashData != "" {
		flashdata, err := LoadPackageFile(archive, binary.FlashData)
		if err != nil {
			L.RaiseError("Couldn't read flashdata: %s", err)
			return 0
		}
		slot.RawSetString("fxdata", lua.LString(string(flashdata)))
		log.Printf("Package %s flashdata: %d bytes", realfilepath, len(flashdata))
	}
	if binary.FlashSave != "" {
		flashsave, err := LoadPackageFile(archive, binary.FlashSave)
		if err != nil {
			L.RaiseError("Couldn't read flashsave: %s", err)
			return 0
		}
		slot.RawSetString("fxsave", lua.LString(string(flashsave)))
		log.Printf("Package %s flashsave: %d bytes", realfilepath, len(flashsave))
	}

	// Try setting an image if one isn't set
	if binary.CartImage == "" {
		binary.CartImage, err = FindSuitablePackageImage(archive)
		if err != nil {
			log.Printf("Error looking for cart image: %s", err)
		}
	}

	// Load the image
	if binary.CartImage != "" {
		imagereader, err := archive.Open(binary.CartImage)
		if err != nil {
			L.RaiseError("Can't open cart image: %s", err)
			return 0
		}
		defer imagereader.Close()
		paletted, err := RawImageToPalettedTitle(imagereader, uint8(threshold))
		if err != nil {
			L.RaiseError("Error converting image to title: %s", err)
			return 0
		}
		raw, _, err := PalettedToRaw(paletted, ScreenWidth, ScreenHeight)
		if err != nil {
			L.RaiseError("Can't convert title raw: %s", err)
			return 0
		}
		slot.RawSetString("image", lua.LString(string(raw)))
		log.Printf("Loaded cart image for package %s: %d bytes", realfilepath, len(raw))
	}

	L.Push(&slot)
	return 1
}

// -----------------------------
//            RUN
// -----------------------------

func RunLuaFlashcartGenerator(script string, arguments []string, dir string) (string, error) {
	state := FlashcartState{
		Readers:       make([]*FlashcartReader, 0),
		Writers:       make([]*FlashcartWriter, 0),
		FileDirectory: dir,
		Arguments:     arguments,
	}

	defer state.CloseAll()

	L := lua.NewState()
	defer L.Close()

	var outputBuffer bytes.Buffer

	setBasicLuaFunctions(L)
	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		for i := 1; i <= n; i++ {
			outputBuffer.WriteString(L.CheckString(i))
			if i != n {
				outputBuffer.WriteString("\t")
			}
		}
		outputBuffer.WriteString("\n")
		return 0
	}))
	L.SetGlobal("is_category", L.NewFunction(luaIsCategory))
	L.SetGlobal("has_fxsave", L.NewFunction(luaHasFxsave))
	state.AddFunction("parse_flashcart", luaParseFlashcart, L)
	state.AddFunction("new_flashcart", luaNewFlashcart, L)
	state.AddFunction("arguments", luaGetArguments, L)
	state.AddFunction("title_image", luaTitleImage, L)
	state.AddFunction("package", func(L *lua.LState, state *FlashcartState) int { return luaPackageReader(L, state, false) }, L)
	state.AddFunction("packageany", func(L *lua.LState, state *FlashcartState) int { return luaPackageReader(L, state, true) }, L)

	err := L.DoString(script)
	if err != nil {
		return "", err
	}

	return string(outputBuffer.Bytes()), nil
}
