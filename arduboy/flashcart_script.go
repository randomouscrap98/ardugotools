package arduboy

import (
	//"fmt"
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
	scanFunc := func(f io.ReadSeeker, header *FxHeader, addr int64, index int) error {
		var slot lua.LTable
		slot.RawSetString("title", lua.LString(header.Title))
		slot.RawSetString("version", lua.LString(header.Version))
		slot.RawSetString("developer", lua.LString(header.Developer))
		slot.RawSetString("info", lua.LString(header.Info))
		slot.RawSetString("is_category", lua.LBool(header.IsCategory()))
		// Read the image too
		pullData := func(L *lua.LState) int {
			// Here, we have access to both the reader and the header. Using these
			// two, we can seek to the right location, then read the data.
			//_, err := f.Seek(header.
			//slot.RawSetString
			return 0
		}
		slot.RawSetString("pull_data", L.NewFunction(pullData))
		result.RawSetInt(index+1, &slot)
		if preload {
			pullData(L)
		}
		return nil
	}
	_, err = ScanFlashcartFile(file, scanFunc)
	if err != nil {
		L.RaiseError("Error scanning flashcart: %s", err)
		return 0
	}
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

// -----------------------------
//            RUN
// -----------------------------

func RunLuaFlashcartGenerator(script string, arguments []string, dir string) error {
	state := FlashcartState{
		Readers:       make([]FlashcartReader, 0),
		Writers:       make([]FlashcartWriter, 0),
		FileDirectory: dir,
	}

	defer state.CloseAll()

	L := lua.NewState()
	defer L.Close()

	setBasicLuaFunctions(L)
	state.AddFunction("parse_flashcart", luaParseFlashcart, L)
	state.AddFunction("new_flashcart", luaNewFlashcart, L)
	// state.AddFunction("file", luaFile, L)
	// state.AddFunction("image", luaImage, L)
	// state.AddFunction("address", luaAddress, L)              // current address
	// state.AddFunction("header", luaHeader, L)                // Write arbitrary header text
	// state.AddFunction("field", luaField, L)                  // Write header definition for field (begin field)
	// state.AddFunction("image_helper", luaImageHelper, L)     // write header stuff for image (begin field)
	// state.AddFunction("raycast_helper", luaRaycastHelper, L) // write header stuff for raycast image (begin field)
	// state.AddFunction("write", luaWrite, L)                  // Write raw data to bin (no header)
	// state.AddFunction("pad", luaPad, L)                      // pad data to given alignment
	// state.AddFunction("begin_save", luaBeginSave, L)         // begin the save section

	err := L.DoString(script)
	if err != nil {
		return err
	}

	return nil
}
