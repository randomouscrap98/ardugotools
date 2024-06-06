package arduboy

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/yuin/gopher-lua"
)

type FxDataState struct {
	Header           io.Writer
	Bin              io.Writer
	SaveLength       int
	DataLength       int
	CurrentNamespace string
	//MinSaveLength    int
	// func ParseFxData(data *FxData, header io.Writer, bin io.Writer) (*FxOffsets, error) {
}

// Write the raw string to the header with the given number of extra newlines. Raises
// a lua "error" if writing the header doesn't work
func (state *FxDataState) WriteHeader(raw string, extraNewlines int, L *lua.LState) {
	for i := 0; i < extraNewlines; i++ {
		raw += "\n"
	}
	_, err := state.Header.Write([]byte(raw))
	if err != nil {
		L.RaiseError("Couldn't write raw header string %s: %s", raw, err)
	}
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
	} else {
		log.Printf("Read %d bytes from file %s in lua script", len(bytes), filename)
		L.Push(lua.LString(string(bytes)))
		return 1
	}
}

// Function for lua scripts that lets you parse hex
func luaHex(L *lua.LState) int {
	hexstring := L.ToString(1)
	bytes, err := hex.DecodeString(hexstring)
	if err != nil {
		L.RaiseError("Error decoding hex in lua script: %s", err)
		return 0
	} else {
		log.Printf("Decoded %d bytes from hex in lua script", len(bytes))
		L.Push(lua.LString(string(bytes)))
		return 1
	}
}

// Function for lua scripts that lets you parse base64
func luaBase64(L *lua.LState) int {
	b64string := L.ToString(1)
	bytes, err := base64.StdEncoding.DecodeString(b64string)
	if err != nil {
		L.RaiseError("Error decoding base64 in lua script: %s", err)
		return 0
	} else {
		log.Printf("Decoded %d bytes from base64 in lua script", len(bytes))
		L.Push(lua.LString(string(bytes)))
		return 1
	}
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
func luaBegin(L *lua.LState, state *FxDataState) int {
	name := L.ToString(1)
	addr := state.DataLength
	state.WriteHeader(fmt.Sprintf("constexpr uint24_t %s = 0x%0*X;\n", name, 6, addr), 0, L)
	L.Push(lua.LNumber(addr)) //(string(bytes)))
	return 1
}

// Allow user to set a fixed save length
func luaFixedSave(L *lua.LState, state *FxDataState) int {
	size := L.ToInt(1)
	state.SaveLength = size
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
	var offsets FxOffsets
	var state FxDataState

	L := lua.NewState()
	defer L.Close()

	L.SetGlobal("file", L.NewFunction(luaFile))
	L.SetGlobal("hex", L.NewFunction(luaHex))
	L.SetGlobal("base64", L.NewFunction(luaBase64))
	L.SetGlobal("json", L.NewFunction(luaJson))
	state.AddFunction("header", luaHeader, L)
	state.AddFunction("preamble", luaPreamble, L)
	state.AddFunction("begin", luaBegin, L)
	state.AddFunction("save_length", luaFixedSave, L)

	err := L.DoString(script)
	if err != nil {
		return nil, err
	}

	return &offsets, nil
}

//ParseFxData(data *FxData, header io.Writer, bin io.Writer) (*FxOffsets, error) {
