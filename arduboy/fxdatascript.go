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

	"github.com/yuin/gopher-lua"
)

// Tracking data for fx script system
type FxDataState struct {
	Header           io.Writer
	Bin              io.Writer
	BinLength        int  // Total bin length as of now
	DataEnd          int  // Exclusive end
	SaveStart        int  // Inclusive start
	HasSave          bool // Whether a save is active for this thing
	CurrentNamespace string
}

func (state *FxDataState) CurrentAddress() int {
	return state.BinLength - state.SaveStart
}

func (state *FxDataState) FinalizeBin() (*FxOffsets, error) {
	var offsets FxOffsets
	if state.HasSave {
		// Having a save means padding only the SAVE data to the correct length
		offsets.DataLength = state.DataEnd
		offsets.DataLengthFlash = state.SaveStart
		offsets.SaveLength = state.BinLength - state.SaveStart // This could be 0, that's fine
		newlength := int(AlignWidth(uint(state.BinLength), uint(FxSaveAlignment)))
		if offsets.SaveLength == 0 {
			newlength += FxSaveAlignment // FORCE save if user has begun save at all
		}
		// Write the save padding. We know data padding is already written if there's a save
		if newlength > state.BinLength {
			_, err := state.Bin.Write(MakePadding(newlength - state.BinLength))
			if err != nil {
				return nil, err
			}
		}
		offsets.SaveLengthFlash = state.BinLength - state.SaveStart
	} else {
		// Having no save means only padding data. Save is always 0 here
		offsets.DataLength = state.BinLength
		newlength := int(AlignWidth(uint(state.BinLength), uint(FXPageSize)))
		if newlength > state.BinLength {
			_, err := state.Bin.Write(MakePadding(newlength - state.BinLength))
			if err != nil {
				return nil, err
			}
		}
		offsets.DataLengthFlash = state.BinLength
	}
	offsets.SaveStart = FxDevExpectedFlashCapacity - offsets.SaveLengthFlash
	offsets.DataStart = offsets.SaveStart - offsets.DataLengthFlash
	return &offsets, nil
}

// Write the raw string to the header with the given number of extra newlines. Raises
// a lua "error" if writing the header doesn't work
func (state *FxDataState) WriteHeader(raw string, extraNewlines int, L *lua.LState) int {
	for i := 0; i < extraNewlines; i++ {
		raw += "\n"
	}
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

// End the data section and begin writting the save section. It's all the same
// to the bin, we just must remember where the save data starts
func (state *FxDataState) BeginSave(L *lua.LState) int {
	// Must align to fx page size
	newlength := int(AlignWidth(uint(state.BinLength), uint(FXPageSize)))
	state.DataEnd = state.BinLength
	state.HasSave = true
	written := 0
	if newlength > state.BinLength {
		written = state.WriteBin(MakePadding(newlength-state.BinLength), L)
	}
	state.SaveStart = state.BinLength
	return written
}

// Shorthand to add global function that also accepts this state
func (state *FxDataState) AddFunction(name string, f func(*lua.LState, *FxDataState) int, L *lua.LState) {
	L.SetGlobal(name, L.NewFunction(func(L *lua.LState) int { return f(L, state) }))
}

// -----------------------------
//          READERS
// -----------------------------

// Function for lua scripts that lets you read an entire file. Yes, that's already
// possible in lua, whatever
func luaFile(L *lua.LState) int {
	filename := L.ToString(1) // First param is the filename
	bytes, err := os.ReadFile(filename)
	if err != nil {
		L.RaiseError("Error reading file %s in lua script: %s", filename, err)
		return 0
	}
	log.Printf("Read %d bytes from file %s in lua script", len(bytes), filename)
	L.Push(lua.LString(string(bytes)))
	return 1
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
	state.WriteHeader(L.ToString(1), L.ToInt(2), L)
	return 0
}

// Write the preamble. Not done by default, in case you don't want it or something...
func luaPreamble(L *lua.LState, state *FxDataState) int {
	state.WriteHeader("#pragma once\n\nusing uint24_t = __uint24;\n\n", 0, L)
	return 0
}

// Begin a new variable. This writes the variable as the current address
// to the header, but does not write any data
func luaField(L *lua.LState, state *FxDataState) int {
	name := L.ToString(1)
	addr := state.CurrentAddress()
	state.WriteHeader(fmt.Sprintf("constexpr uint24_t %s = 0x%0*X;\n", name, 6, addr), 0, L)
	L.Push(lua.LNumber(addr))
	return 1
}

// Begin the save section. Simply beginning save will set that there IS a save
func luaBeginSave(L *lua.LState, state *FxDataState) int {
	state.BeginSave(L)
	return 0
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

// // A super wasteful function to append a 0 to the end of the data.
// func luaString(L *lua.LState) int {
// 	s := L.ToString(1)
//   b := append([]byte(s), 0)
//   L.Push(lua.LString(string(b)))
//   return 1
// }

// Run an entire lua script which may write fxdata to the given header and bin files.
func RunLuaFxGenerator(script string, header io.Writer, bin io.Writer) (*FxOffsets, error) {
	state := FxDataState{
		Header: header,
		Bin:    bin,
	}

	L := lua.NewState()
	defer L.Close()

	L.SetGlobal("file", L.NewFunction(luaFile))
	L.SetGlobal("hex", L.NewFunction(luaHex))
	L.SetGlobal("base64", L.NewFunction(luaBase64))
	L.SetGlobal("json", L.NewFunction(luaJson))
	L.SetGlobal("bytes", L.NewFunction(luaBytes))
	state.AddFunction("header", luaHeader, L)
	state.AddFunction("preamble", luaPreamble, L)
	state.AddFunction("pad", luaPad, L)
	state.AddFunction("field", luaField, L)
	state.AddFunction("begin_save", luaBeginSave, L)

	err := L.DoString(script)
	if err != nil {
		return nil, err
	}

	// Some final calcs based on how much data we wrote
	offsets, err := state.FinalizeBin()
	if err != nil {
		return nil, err
	}

	return offsets, nil
}
