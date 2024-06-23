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
	File *os.File
	//Address int // Current address within the flashcart
	// TODO: add settings for menu, contrast, screen patching, etc
}

// General tracking for entire lua script (user can open arbitrary flashcarts)
type FlashcartState struct {
	FileDirectory string
	Readers       []FlashcartReader
	Writers       []FlashcartWriter
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
	state.Readers = append(state.Readers, FlashcartReader{
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
		slot.RawSetString("pull_data", L.NewFunction(func(L *lua.LState) int {
			err := pullData()
			if err != nil {
				L.RaiseError("Error pulling data from slot #%d at %x: %s", index, addr, err)
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
	// Attempt to open the file first.
	file, err := os.Create(filepath)
	if err != nil {
		L.RaiseError("Error creating new flashcart: %s", err)
		return 0
	}
	// Now that we have a working file, we must immediately add it to the
	// writers. The writers list is automatically cleaned up
	state.Writers = append(state.Writers, FlashcartWriter{
		File: file,
	})
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
		Readers:       make([]FlashcartReader, 0),
		Writers:       make([]FlashcartWriter, 0),
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
