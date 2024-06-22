package arduboy

import (
	//"fmt"
	//"io"
	"log"
	"os"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
)

type FlashcartReader struct {
	File    *os.File
	Address int // Current address within the flashcart
}

type FlashcartWriter struct {
	File    *os.File
	Address int // Current address within the flashcart
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
