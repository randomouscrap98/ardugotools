package arduboy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

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
	LastSlotAddr int
	//Address int // Current address within the flashcart
	// TODO: add settings for menu, contrast, screen patching, etc
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
	if state.FileDirectory == "" {
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
		err := f.File.Close()
		if err != nil {
			log.Printf("ERROR: Couldn't close pending writer: %s", err)
			results = append(results, err)
		}
	}
	return results
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
	header := FxHeader{
		PreviousPage: uint16(writer.LastSlotAddr / FXPageSize),
		NextPage:     0xFFFF,
		ProgramStart: 0xFFFF,
		DataStart:    0xFFFF,
		SaveStart:    0xFFFF,
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
	is_category := len(sketch) == 0
	if len(image) != FxHeaderImageLength {
		L.RaiseError("Invalid image length on slot %d!", writer.Slots)
		return 0
	}
	if is_category {
		header.Category = uint8(writer.CategoryId)
		writer.CategoryId += 1
	} else if writer.Slots < 2 {
		L.RaiseError("First two slots MUST be categories! ")
		return 0
	}
	headerraw, err := header.MakeHeader()
	if err != nil {
		L.RaiseError("Couldn't compile header: %s", err)
		return 0
	}
	sw := func(data []byte) bool {
		if len(data) > 0 {
			_, err := writer.File.Write(data)
			if err != nil {
				L.RaiseError("Couldn't write to flashcart: %s", err)
				return true
			}
		}
		return false
	}
	if sw(headerraw) || sw(image) || sw(sketch) || sw(fxdata) || sw(fxsave) {
		return 0
	}
	writer.Slots += 1
	writer.LastSlotAddr = int(addr)
	L.Push(lua.LNumber(int(header.SlotPages) * FXPageSize))
	return 1
}

// -----------------------------
//          FUNCTIONS
// -----------------------------

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
		slot.RawSetString("is_category", lua.LBool(header.IsCategory()))
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
	filepath := state.FilePath(relpath)
	// Attempt to create the file first.
	file, err := os.Create(filepath)
	if err != nil {
		L.RaiseError("Error creating new flashcart: %s", err)
		return 0
	}
	// Now that we have a working file, we must immediately add it to the
	// writers. The writers list is automatically cleaned up
	writer := FlashcartWriter{
		File:         file,
		LastSlotAddr: 0xFFFF, // first slot always has this as 0xFFFF
	}
	state.Writers = append(state.Writers, &writer)
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
	state.AddFunction("parse_flashcart", luaParseFlashcart, L)
	state.AddFunction("new_flashcart", luaNewFlashcart, L)
	state.AddFunction("arguments", luaGetArguments, L)

	err := L.DoString(script)
	if err != nil {
		return "", err
	}

	return string(outputBuffer.Bytes()), nil
}
