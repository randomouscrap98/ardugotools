package arduboy

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/gopher-lua"
)

const (
	FxDevExpectedFlashCapacity = 1 << 24
)

type FxOffsets struct {
	DataLength      int // real length of data as user defined it
	SaveLength      int // real length of save as user defined it
	DataLengthFlash int // length of data on flash (may be larger than DataLength)
	SaveLengthFlash int // length of save on flash (may be larger than SaveLength)
	DataStart       int // Beginning address (byte) of data
	SaveStart       int // Beginning address (byte) of save (will be past end of flash if no save)
}

// Tracking data for fx script system
type FxDataState struct {
	Header           io.Writer
	Bin              io.Writer
	BinLength        int  // Total bin length as of now
	DataEnd          int  // Exclusive end
	SaveStart        int  // Inclusive start
	HasSave          bool // Whether a save is active for this thing
	CurrentNamespace string
	FileDirectory    string
}

func (state *FxDataState) CurrentAddress() int {
	return state.BinLength - state.SaveStart
}

func (state *FxDataState) FilePath(path string) string {
	if state.FileDirectory == "" {
		return path
	}
	return filepath.Join(state.FileDirectory, path)
}

func (state *FxDataState) FinalizeBin() (*FxOffsets, error) {
	var offsets FxOffsets
	log.Printf("Ending fx data generation. Total length: %d (save: %t)",
		state.BinLength, state.HasSave)
	if state.HasSave {
		// Having a save means padding only the SAVE data to the correct length
		offsets.DataLength = state.DataEnd
		offsets.DataLengthFlash = state.SaveStart
		offsets.SaveLength = state.BinLength - state.SaveStart // This could be 0, that's fine
		newlength := state.SaveStart + int(AlignWidth(uint(offsets.SaveLength), uint(FxSaveAlignment)))
		if offsets.SaveLength == 0 {
			newlength += FxSaveAlignment // FORCE save if user has begun save at all
		}
		// Write the save padding. We know data padding is already written if there's a save
		if newlength > state.BinLength {
			written, err := state.Bin.Write(MakePadding(newlength - state.BinLength))
			if err != nil {
				return nil, err
			}
			state.BinLength += written
		}
		offsets.SaveLengthFlash = state.BinLength - state.SaveStart
	} else {
		// Having no save means only padding data. Save is always 0 here
		offsets.DataLength = state.BinLength
		newlength := int(AlignWidth(uint(state.BinLength), uint(FXPageSize)))
		if newlength > state.BinLength {
			written, err := state.Bin.Write(MakePadding(newlength - state.BinLength))
			if err != nil {
				return nil, err
			}
			state.BinLength += written
		}
		offsets.DataLengthFlash = state.BinLength
	}
	offsets.SaveStart = FxDevExpectedFlashCapacity - offsets.SaveLengthFlash
	offsets.DataStart = offsets.SaveStart - offsets.DataLengthFlash
	return &offsets, nil
}

// Write the raw string to the header with the given number of extra newlines. Raises
// a lua "error" if writing the header doesn't work
func (state *FxDataState) WriteHeader(raw string /*extraNewlines int,*/, L *lua.LState) int {
	written, err := state.Header.Write([]byte(raw))
	if err != nil {
		L.RaiseError("Couldn't write raw header string %s: %s", raw, err)
	}
	return written
}

// Write the raw data directly to the bin. Pretty simple! But raises a script error
// if there's an error in the underlying write
func (state *FxDataState) WriteBin(raw []byte, L *lua.LState) int {
	written, err := state.Bin.Write(raw)
	if err != nil {
		L.RaiseError("Couldn't write raw binary of %d bytes: %s", len(raw), err)
	}
	state.BinLength += written
	return written
}

// Shorthand to add global function that also accepts this state
func (state *FxDataState) AddFunction(name string, f func(*lua.LState, *FxDataState) int, L *lua.LState) {
	L.SetGlobal(name, L.NewFunction(func(L *lua.LState) int { return f(L, state) }))
}

// -----------------------------
//          HELPERS
// -----------------------------

func MakeFxHeaderField(typ string, name string, value int, hex int) string {
	if hex > 0 {
		return fmt.Sprintf("constexpr %s %s = 0x%0*X;\n", typ, name, hex, value)
	} else {
		return fmt.Sprintf("constexpr %s %s = %d;\n", typ, name, value)
	}
}

// Return the line representing the full field at the given address.
// Only works for actual fxdata (don't use for FX_DATA_PAGE etc)
func MakeFxHeaderAddress(name string, addr int) string {
	return MakeFxHeaderField("uint24_t", name, addr, 6)
}

// Return the block representing a main fx pointer, such as FX_DATA_PAGE
// or FX_SAVE_PAGE
func MakeFxHeaderMainPointer(name string, addr uint, length uint) string {
	return fmt.Sprintf("%s%s\n",
		MakeFxHeaderField("uint16_t", name+"_PAGE", int(addr)/FXPageSize, 4),
		MakeFxHeaderField("uint24_t", name+"_BYTES", int(length), 0))
}

// -----------------------------
//          READERS
// -----------------------------

// Return the current address pointed to in the system. Knows whether it's
// save or data
func luaAddress(L *lua.LState, state *FxDataState) int {
	L.Push(lua.LNumber(state.CurrentAddress()))
	return 1
}

// Function for lua scripts that lets you read an entire file. Yes, that's already
// possible in lua, whatever
func luaFile(L *lua.LState, state *FxDataState) int {
	filename := L.ToString(1) // First param is the filename
	bytes, err := os.ReadFile(state.FilePath(filename))
	if err != nil {
		L.RaiseError("Error reading file %s in lua script: %s", filename, err)
		return 0
	}
	log.Printf("Read %d bytes from file %s in lua script", len(bytes), filename)
	L.Push(lua.LString(string(bytes)))
	return 1
}

// Generates raw image data, width, height, and frames as return data.
// The user can do whatever they want with it
func luaImage(L *lua.LState, state *FxDataState) int {

	// TODO: Make sure you generate a help thing for this!
	filename := L.ToString(1)
	width := L.ToInt(2)       // Width of tile (0 means use all available width)
	height := L.ToInt(3)      // Height of tile (0 means use all available height)
	spacing := L.ToInt(4)     // Spacing between tiles (including on edges)
	usemask := L.ToBool(5)    // Whether to use transparency as a data mask
	threshold := L.ToInt(6)   // The upper bound for black pixels
	alphathresh := L.ToInt(7) // The upper bound for alpha threshold

	// Validation and/or setting the defaults if not set
	if filename == "" {
		L.RaiseError("Must provide filename for image!")
		return 0
	}
	if threshold == 0 {
		threshold = 100
	}
	if alphathresh == 0 {
		alphathresh = 10
	}

	file, err := os.Open(state.FilePath(filename))
	if err != nil {
		L.RaiseError("Error opening image file: %s", err)
		return 0
	}
	defer file.Close()
	tc := TileConfig{
		Width:   width,
		Height:  height,
		Spacing: spacing,
		UseMask: usemask,
	}
	tiles, computed, err := SplitImageToTiles(file, &tc)
	if err != nil {
		L.RaiseError("Error splitting image to tiles: %s", err)
		return 0
	}

	// Buffer for the whole data, as in the entire thing for images.
	// We don't check for errors here because... well, CAN an in-memory
	// buffer throw errors? I'd be surprised...
	var buf bytes.Buffer
	onebyte := make([]byte, 1)

	// Need to write the width and height as 2 byte fields
	preamble := make([]byte, 4)
	Write2ByteValue(uint16(computed.SpriteWidth), preamble, 0)
	Write2ByteValue(uint16(computed.SpriteHeight), preamble, 2)
	buf.Write(preamble)

	// Now write all the tiles
	for i, tile := range tiles {
		ptile, w, h := ImageToPaletted(tile, uint8(threshold), uint8(alphathresh))
		raw, mask, err := PalettedToRaw(ptile, w, h)
		if err != nil {
			L.RaiseError("Can't convert tile %d to raw: %s", i, err)
			return 0
		}
		for i := range raw {
			onebyte[0] = raw[i]
			buf.Write(onebyte)
			if usemask {
				onebyte[0] = mask[i]
				buf.Write(onebyte)
			}
		}
	}
	bytes := buf.Bytes()
	log.Printf("Converted image '%s' to %d tiles of %d width, %d height (%d bytes)",
		filename, len(tiles), computed.SpriteWidth, computed.SpriteHeight, len(bytes))

	// TODO: document the return types!!!
	L.Push(lua.LString(string(bytes)))         // Actual raw data
	L.Push(lua.LNumber(len(tiles)))            // amount of tiles
	L.Push(lua.LNumber(computed.SpriteWidth))  // individual sprite width
	L.Push(lua.LNumber(computed.SpriteHeight)) // individual sprite height

	return 4
}

// Function for lua scripts that lets you parse hex
func luaHex(L *lua.LState) int {
	hexstring := L.ToString(1)
	bytes, err := hex.DecodeString(hexstring)
	if err != nil {
		L.RaiseError("Error decoding hex in lua script: %s", err)
		return 0
	}
	log.Printf("Decoded %d bytes from hex in lua script", len(bytes))
	L.Push(lua.LString(string(bytes)))
	return 1
}

// Function for lua scripts that lets you parse base64
func luaBase64(L *lua.LState) int {
	b64string := L.ToString(1)
	bytes, err := base64.StdEncoding.DecodeString(b64string)
	if err != nil {
		L.RaiseError("Error decoding base64 in lua script: %s", err)
		return 0
	}
	log.Printf("Decoded %d bytes from base64 in lua script", len(bytes))
	L.Push(lua.LString(string(bytes)))
	return 1
}

// Takes a byte array and turns it into the general writable type (string)
func luaBytes(L *lua.LState) int {
	table := L.ToTable(1)
	typ := L.ToString(2)
	if table == nil {
		L.RaiseError("Error: must pass a table!")
		return 0
	}
	var buf bytes.Buffer
	var err error
	writebuf := func(d any) {
		err = binary.Write(&buf, binary.LittleEndian, d)
	}
	for i := 1; i <= table.Len(); i++ {
		lv := table.RawGetInt(i)
		if num, ok := lv.(lua.LNumber); ok {
			raw := float64(num)
			if typ == "float64" {
				writebuf(raw)
			} else if typ == "float32" {
				writebuf(float32(raw))
			} else if typ == "int32" {
				writebuf(int32(raw))
			} else if typ == "uint32" {
				writebuf(uint32(raw))
			} else if typ == "int16" {
				writebuf(int16(raw))
			} else if typ == "uint16" {
				writebuf(uint16(raw))
			} else if typ == "int8" {
				writebuf(int8(raw))
			} else if typ == "uint8" {
				writebuf(uint8(raw))
			} else if typ == "byte" || typ == "" {
				writebuf(byte(raw))
			} else {
				L.RaiseError("Unknown type: %s", typ)
				return 0
			}
			if err != nil {
				L.RaiseError("Error converting array to bytes: %s", err)
				return 0
			}
		} else {
			L.RaiseError("Error: index %d must be a number!", i)
			return 0
		}
	}
	bytes := buf.Bytes()
	log.Printf("Encoded %d bytes from raw in lua script", len(bytes))
	L.Push(lua.LString(string(bytes)))
	return 1
}

// Simple function to decode a string into a lua table. Returns the table.
// Raises script error on any error.
func luaJson(L *lua.LState) int {
	str := L.ToString(1)
	var value interface{}
	err := json.Unmarshal([]byte(str), &value)
	if err != nil {
		L.RaiseError("Couldn't parse json: %s", err)
		return 0
	}
	L.Push(luaDecodeValue(L, value))
	log.Printf("Decoded json to table in lua script")
	return 1
}

// DecodeValue converts the value to a Lua value.
// Taken from https://github.com/layeh/gopher-json
// This function only converts values that the encoding/json package decodes to.
// All other values will return lua.LNil.
func luaDecodeValue(L *lua.LState, value interface{}) lua.LValue {
	switch converted := value.(type) {
	case bool:
		return lua.LBool(converted)
	case float64:
		return lua.LNumber(converted)
	case string:
		return lua.LString(converted)
	case json.Number:
		return lua.LString(converted)
	case []interface{}:
		arr := L.CreateTable(len(converted), 0)
		for _, item := range converted {
			arr.Append(luaDecodeValue(L, item))
		}
		return arr
	case map[string]interface{}:
		tbl := L.CreateTable(0, len(converted))
		for key, item := range converted {
			tbl.RawSetH(lua.LString(key), luaDecodeValue(L, item))
		}
		return tbl
	case nil:
		return lua.LNil
	}

	return lua.LNil
}

// -----------------------------
//          WRITERS
// -----------------------------

// Write raw text to the header. You can use this to start a namespace
// or whatever
func luaHeader(L *lua.LState, state *FxDataState) int {
	state.WriteHeader(L.ToString(1), L)
	return 0
}

// Begin a new variable. This writes the variable as the current address
// to the header, but does not write any data
func luaField(L *lua.LState, state *FxDataState) int {
	name := L.ToString(1)
	addr := state.CurrentAddress()
	state.WriteHeader(fmt.Sprintf("constexpr uint24_t %s = 0x%0*X;\n", name, 6, addr), L)
	L.Push(lua.LNumber(addr))
	return 1
}

// Helper function to write both to the header and the data
func luaImageHelper(L *lua.LState, state *FxDataState) int {
	name := L.ToString(1)
	data := L.ToString(2)
	frames := L.ToNumber(3)
	width := L.ToNumber(4)
	height := L.ToNumber(5)
	addr := state.CurrentAddress()
	// Write all the normal header stuff
	state.WriteHeader(fmt.Sprintf("// Image info for \"%s\"\n", name), L)
	state.WriteHeader(fmt.Sprintf("constexpr uint24_t %s       = 0x%0*X;\n", name, 6, addr), L)
	state.WriteHeader(fmt.Sprintf("constexpr uint8_t  %sFrames = %d;\n", name, frames), L)
	state.WriteHeader(fmt.Sprintf("constexpr uint16_t %sWidth  = %d;\n", name, width), L)
	state.WriteHeader(fmt.Sprintf("constexpr uint16_t %sHeight = %d;\n", name, height), L)
	// Write the data
	count := state.WriteBin([]byte(data), L)
	// Return both the address AND the length
	L.Push(lua.LNumber(addr))
	L.Push(lua.LNumber(count))
	return 2
}

// End the data section and begin writting the save section. It's all the same
// to the bin, we just must remember where the save data starts
func luaBeginSave(L *lua.LState, state *FxDataState) int {
	if state.HasSave {
		L.RaiseError("Save already begun!")
		return 0
	}
	// Must align to fx page size
	newlength := int(AlignWidth(uint(state.BinLength), uint(FXPageSize)))
	state.DataEnd = state.BinLength
	state.HasSave = true
	written := 0
	if newlength > state.BinLength {
		written = state.WriteBin(MakePadding(newlength-state.BinLength), L)
	}
	state.SaveStart = state.BinLength
	log.Printf("Began save at addr 0x%06X, data ends at 0x%06X", state.SaveStart, state.DataEnd)
	L.Push(lua.LNumber(written))
	return 1
}

// Write the given bytes to the binary. You can do this at any time
func luaWrite(L *lua.LState, state *FxDataState) int {
	data := L.ToString(1)
	count := state.WriteBin([]byte(data), L)
	L.Push(lua.LNumber(count))
	return 1
}

// Pad data at THIS point to be aligned to a certain width. This is OVERALL data
func luaPad(L *lua.LState, state *FxDataState) int {
	align := L.ToInt(1)
	increase := L.ToBool(2)
	newlength := int(AlignWidth(uint(state.BinLength), uint(align)))
	if newlength == state.BinLength && increase {
		newlength += align
	}
	if newlength > state.BinLength {
		log.Printf("Padding data to %d alignment: %d -> %d", align, state.BinLength, newlength)
		state.WriteBin(MakePadding(newlength-state.BinLength), L)
	}
	return 0
}

// -----------------------------
//           STATE
// -----------------------------

// Run an entire lua script which may write fxdata to the given header and bin files.
// For files loaded from the script, load them from dir (or send empty string for nothing)
func RunLuaFxGenerator(script string, header io.Writer, bin io.Writer, dir string) (*FxOffsets, error) {
	state := FxDataState{
		Header:        header,
		Bin:           bin,
		FileDirectory: dir,
	}

	L := lua.NewState()
	defer L.Close()

	L.SetGlobal("hex", L.NewFunction(luaHex))
	L.SetGlobal("base64", L.NewFunction(luaBase64))
	L.SetGlobal("json", L.NewFunction(luaJson))
	L.SetGlobal("bytes", L.NewFunction(luaBytes))
	state.AddFunction("file", luaFile, L)
	state.AddFunction("image", luaImage, L)
	state.AddFunction("address", luaAddress, L)          // current address
	state.AddFunction("header", luaHeader, L)            // Write arbitrary header text
	state.AddFunction("field", luaField, L)              // Write header definition for field (begin field)
	state.AddFunction("image_helper", luaImageHelper, L) // write header stuff for image (begin field)
	state.AddFunction("write", luaWrite, L)              // Write raw data to bin (no header)
	state.AddFunction("pad", luaPad, L)                  // pad data to given alignment
	state.AddFunction("begin_save", luaBeginSave, L)     // begin the save section

	// Always write the preamble before the user starts...
	_, err := io.WriteString(state.Header, fmt.Sprintf(`#pragma once

using uint24_t = __uint24;

// Generated with ardugotools on %s

`, time.Now().Format(time.RFC3339)))
	if err != nil {
		return nil, err
	}

	err = L.DoString(script)
	if err != nil {
		return nil, err
	}

	// Some final calcs based on how much data we wrote
	offsets, err := state.FinalizeBin()
	if err != nil {
		return nil, err
	}

	// Some header finalization. Don't make the user write this, it only makes sense after
	// computing the final offsets.
	var sb strings.Builder

	sb.WriteString("\n// FX addresses (only really used for initialization)\n")
	sb.WriteString(MakeFxHeaderMainPointer("FX_DATA", uint(offsets.DataStart), uint(offsets.DataLength)))
	if max(offsets.SaveLengthFlash) > 0 {
		sb.WriteString(MakeFxHeaderMainPointer("FX_SAVE", uint(offsets.SaveStart), uint(offsets.SaveLength)))
	}
	sb.WriteString("// Helper macro to initialize fx, call in setup()\n")
	if state.HasSave {
		sb.WriteString("#define FX_INIT() FX::begin(FX_DATA_PAGE, FX_DATA_SAVE)\n")
	} else {
		sb.WriteString("#define FX_INIT() FX::begin(FX_DATA_PAGE)\n")
	}

	_, err = io.WriteString(state.Header, sb.String())
	if err != nil {
		return nil, err
	}

	return offsets, nil
}
