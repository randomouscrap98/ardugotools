package arduboy

// General lua scripting functions.

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml"

	lua "github.com/yuin/gopher-lua"
)

func pullString(table *lua.LTable, key string, done func(string)) bool {
	ttemp := table.RawGetString(key)
	tstring, ok := ttemp.(lua.LString)
	if ok {
		done(string(tstring))
	}
	return ok
}

func pullInt(table *lua.LTable, key string, done func(int)) bool {
	ttemp := table.RawGetString(key)
	tnum, ok := ttemp.(lua.LNumber)
	if ok {
		done(int(tnum))
	}
	return ok
}

func pullBool(table *lua.LTable, key string, done func(bool)) bool {
	ttemp := table.RawGetString(key)
	tbool, ok := ttemp.(lua.LBool)
	if ok {
		done(bool(tbool))
	}
	return ok
}

// -----------------------------
//          READERS
// -----------------------------

// A VERY SIMPLE image resize function.
func luaImageResize(L *lua.LState) int {
	data := L.ToTable(1)
	owidth := L.ToInt(2)  // Width of orig tiles
	oheight := L.ToInt(3) // Height of orig tiles
	width := L.ToInt(4)   // New desired width
	height := L.ToInt(5)  // New desired height

	var result lua.LTable
	for i := 1; i <= data.Len(); i++ {
		lv := data.RawGetInt(i)
		if lvstring, ok := lv.(lua.LString); ok {
			raw := []byte(string(lvstring))
			out := make([]byte, width*height)
			for x := 0; x < width; x++ {
				hofs := int(math.Floor(0.5 + float64((owidth-1)*x/(width-1))))
				for y := 0; y < height; y++ {
					vofs := int(math.Floor(0.5 + float64((oheight-1)*y/(height-1))))
					out[x+y*width] = raw[hofs+vofs*owidth]
				}
			}
			result.RawSetInt(i, lua.LString(string(out)))
		} else {
			L.RaiseError("Expected raw tile data at index %d!", i)
		}
	}

	L.Push(&result)
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
			} else if typ == "uint24" {
				// Uint24 is a WHOLE thing...
				var tempbuf bytes.Buffer
				err = binary.Write(&tempbuf, binary.LittleEndian, uint32(raw))
				if err == nil {
					fullbytes := tempbuf.Bytes()
					if len(fullbytes) != 4 {
						L.RaiseError("ARDUGOTOOLS PROGRAMMING ERROR: incorrect uint24 size!")
						return 0
					}
					// Since it's little endian, we cut off the last byte (the most significant bits)
					_, err = buf.Write(fullbytes[:3])
				}
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

// Simple function to decode a toml string into a lua table. Returns the table.
func luaToml(L *lua.LState) int {
	str := L.ToString(1)
	var value interface{}
	err := toml.Unmarshal([]byte(str), &value)
	if err != nil {
		L.RaiseError("Couldn't parse toml: %s", err)
		return 0
	}
	L.Push(luaDecodeValue(L, value))
	log.Printf("Decoded toml to table in lua script")
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
	case int64: // NOTE: wasn't needed for json, needed for toml
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

func luaHex2Bin(L *lua.LState) int {
	sketchhex := L.ToString(1)
	sketch := strings.NewReader(sketchhex)
	bin, err := HexToBin(sketch)
	if err != nil {
		L.RaiseError("Couldn't convert hex to bin: %s", err)
		return 0
	}
	L.Push(lua.LString(string(bin)))
	return 1
}

// Get basic info about the entries in a directory, in "filesystem" order
func luaListDir(L *lua.LState) int {
	path := L.ToString(1)
	entries, err := os.ReadDir(path)
	if err != nil {
		L.RaiseError("Couldn't read directory: %s", err)
		return 0
	}
	var result lua.LTable
	for i, entry := range entries {
		var entrytable lua.LTable
		name := entry.Name()
		thispath := filepath.Join(path, name)
		fullpath, err := filepath.Abs(thispath)
		if err != nil {
			L.RaiseError("Couldn't get abs path of %s: %s", thispath, err)
			return 0
		}
		entrytable.RawSetString("name", lua.LString(name))
		entrytable.RawSetString("path", lua.LString(fullpath))
		entrytable.RawSetString("is_directory", lua.LBool(entry.IsDir()))
		result.RawSetInt(i+1, &entrytable)
	}
	L.Push(&result)
	return 1
}

func setBasicLuaFunctions(L *lua.LState) {
	L.SetGlobal("hex", L.NewFunction(luaHex))
	L.SetGlobal("hex2bin", L.NewFunction(luaHex2Bin))
	L.SetGlobal("base64", L.NewFunction(luaBase64))
	L.SetGlobal("json", L.NewFunction(luaJson))
	L.SetGlobal("toml", L.NewFunction(luaToml))
	L.SetGlobal("bytes", L.NewFunction(luaBytes))
	L.SetGlobal("listdir", L.NewFunction(luaListDir))
	L.SetGlobal("image_resize", L.NewFunction(luaImageResize))
}
