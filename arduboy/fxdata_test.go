package arduboy

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestRunLuaFxGenerator_Empty(t *testing.T) {
	script := "-- Nothing here!"

	var header bytes.Buffer
	var bin bytes.Buffer

	_, err := RunLuaFxGenerator(script, &header, &bin, "")
	if err != nil {
		t.Fatalf("Error running basic fx generator: %s", err)
	}

	headerstr := header.String()

	expected := []string{
		"#pragma once",
		"FX_DATA_PAGE",
		"FX_DATA_BYTES",
	}
	for _, exp := range expected {
		if !strings.Contains(headerstr, exp) {
			t.Fatalf("Didn't write '%s' in empty header. Header:\n%s", exp, headerstr)
		}
	}
}

func TestRunLuaFxGenerator_SaveOnly(t *testing.T) {
	script := "begin_save()"

	var header bytes.Buffer
	var bin bytes.Buffer

	offsets, err := RunLuaFxGenerator(script, &header, &bin, "")
	if err != nil {
		t.Fatalf("Error running saveonly fx generator: %s", err)
	}

	if offsets.DataLength != 0 {
		t.Fatalf("Expected no data in saveonly, got %d", offsets.DataLength)
	}
	if offsets.DataLengthFlash != 0 {
		t.Fatalf("Expected no data(flash) in saveonly, got %d", offsets.DataLengthFlash)
	}
	if offsets.SaveLength != 0 {
		t.Fatalf("Expected no real save in saveonly, got %d", offsets.SaveLength)
	}
	if offsets.SaveLengthFlash != FxSaveAlignment {
		t.Fatalf("Expected %d save(flash) in saveonly, got %d", FxSaveAlignment, offsets.SaveLengthFlash)
	}
	expected := FxDevExpectedFlashCapacity - FxSaveAlignment
	if offsets.DataStart != expected {
		t.Fatalf("Expected DataStart=%d, got %d", expected, offsets.DataStart)
	}
	if offsets.SaveStart != expected {
		t.Fatalf("Expected SaveStart=%d, got %d", expected, offsets.SaveStart)
	}

	headerstr := header.String()

	expectedheaders := []string{
		"#pragma once",
		"FX_DATA_PAGE",
		"FX_DATA_BYTES",
		"FX_SAVE_PAGE",
		"FX_SAVE_BYTES",
	}
	for _, exp := range expectedheaders {
		if !strings.Contains(headerstr, exp) {
			t.Fatalf("Didn't write '%s' in header. Header:\n%s", exp, headerstr)
		}
	}
}

// Run through some of the easier to test converters
func TestRunLuaFxGenerator_Basic(t *testing.T) {
	script := `
-- Let's say we want to support namespaces... it's like this
header("namespace AmazingData {\n")
-- Some weird increasing hex. 17 bytes
field("myhex")
write(hex("000102030405060708090A0B0C0D0E0F10"))
-- base64 of "Hello world!". 12 bytes (no null terminator)
field("mybase64")
write(base64("SGVsbG8gd29ybGQh"))
-- string we write directly, including the null terminator. 40 bytes + 1 (null terminator)
field("mystring")
write("owo uwu !@#$%^&*()-_[]{}|;:?/.><,+=~Z188\0")
-- Raw bytes written directly. 4 bytes
field("myrawbytes")
write(bytes({5, 6, 7, 8}))
-- Raw float32 written directly. 12 bytes
field("myrawfloats")
write(bytes({1.2, -99.9, 0.05071}, "float32"))
-- Raw uint32 written directly. 20 bytes
field("myrawints")
write(bytes({8432, 4320, 432, 85, 1010104}, "uint32"))
-- Raw int16 written directly. 6 bytes
field("myrawshorts")
write(bytes({66, -789, 10405}, "int16"))
header("}\n")
`

	var header bytes.Buffer
	var bin bytes.Buffer

	offsets, err := RunLuaFxGenerator(script, &header, &bin, "")
	if err != nil {
		t.Fatalf("Error running basic fx generator: %s", err)
	}

	expectedDataLength := 112
	if offsets.DataLength != expectedDataLength {
		t.Fatalf("Expected DataLength=%d, got %d", expectedDataLength, offsets.DataLength)
	}
	if offsets.DataLengthFlash != FXPageSize {
		t.Fatalf("Expected DataLengthFlash=%d, got %d", FXPageSize, offsets.DataLengthFlash)
	}

	headerstr := header.String()
	bytes := bin.Bytes()

	if len(bytes) != FXPageSize {
		t.Fatalf("Expected %d bytes, got %d", FXPageSize, len(bytes))
	}

	expectedheaders := []string{
		"namespace AmazingData {",
		"constexpr uint24_t myhex = 0x000000;",
		"constexpr uint24_t mybase64 = 0x000011;",
		"constexpr uint24_t mystring = 0x00001D;",
		"constexpr uint24_t myrawbytes = 0x000046;",
		"constexpr uint24_t myrawfloats = 0x00004A;",
		"constexpr uint24_t myrawints = 0x000056;",
		"constexpr uint24_t myrawshorts = 0x00006A;", // 106 -> 112
	}
	for _, exp := range expectedheaders {
		if !strings.Contains(headerstr, exp) {
			t.Fatalf("Didn't write '%s' in header. Header:\n%s", exp, headerstr)
		}
	}
}

func TestRunLuaFxGenerator_Raycast(t *testing.T) {
	script := `
local written = raycast_helper("spritesheet", true, image("spritesheet.png", 32, 32, 0, true, 100, 10, true))
header("// Raycast bytes written: " .. written .. "\n")
written = raycast_helper("sprootsheet", false, image("spritesheet.png", 32, 32, 0, true, 100, 10, true))
header("// Raycast bytes written2: " .. written .. "\n")
`

	var header bytes.Buffer
	var bin bytes.Buffer

	offsets, err := RunLuaFxGenerator(script, &header, &bin, testPath())
	if err != nil {
		t.Fatalf("Error running basic fx generator: %s", err)
	}

	expectedDataLength := 416 * 4 * 3
	if offsets.DataLength != expectedDataLength {
		t.Fatalf("Expected DataLength=%d, got %d", expectedDataLength, offsets.DataLength)
	}

	headerstr := header.String()
	//bytes := bin.Bytes()

	expectedheaders := []string{
		"constexpr uint24_t spritesheet",
		"constexpr uint24_t spritesheetMask",
		"spritesheetWidth  = 32",
		"spritesheetHeight = 32",
		"spritesheetFrames = 4",
		"sprootsheetWidth  = 32",
		"sprootsheetHeight = 32",
		"sprootsheetFrames = 4",
		"constexpr uint24_t sprootsheet",
		fmt.Sprintf("Raycast bytes written: %d", 416*4*2),
		fmt.Sprintf("Raycast bytes written2: %d", 416*4),
	}
	for _, exp := range expectedheaders {
		if !strings.Contains(headerstr, exp) {
			t.Fatalf("Didn't write '%s' in header. Header:\n%s", exp, headerstr)
		}
	}
	if strings.Contains(headerstr, "sprootsheetMask") {
		t.Fatalf("Unexpected write '%s' in header. Header:\n%s", "sprootsheetMask", headerstr)
	}
}

func TestRunLuaFxGenerator_Real1(t *testing.T) {
	script, err := os.ReadFile(fileTestPath("fxdata.lua"))
	if err != nil {
		t.Fatalf("Couldn't read fxdata.lua for testing: %s", err)
	}

	var header bytes.Buffer
	var bin bytes.Buffer
	offsets, err := RunLuaFxGenerator(string(script), &header, &bin, testPath())

	if err != nil {
		t.Fatalf("Couldn't parse Real1 lua script: %s", err)
	}

	// This is the exact length of the uneven.bin (perhaps programmatically read this?)
	if offsets.SaveLength != 1031 {
		t.Fatalf("Expected savelength 1031, got %d", offsets.SaveLength)
	}
	if offsets.SaveLengthFlash != FxSaveAlignment {
		t.Fatalf("Expected flash savelength %d, got %d", FxSaveAlignment, offsets.SaveLengthFlash)
	}
	expectedSaveLoc := FxDevExpectedFlashCapacity - FxSaveAlignment
	if offsets.SaveStart != expectedSaveLoc {
		t.Fatalf("Expected save location %d, got %d", expectedSaveLoc, offsets.SaveStart)
	}

	if offsets.DataLength != 1099 {
		t.Fatalf("Expected datalength 1099, got %d", offsets.DataLength)
	}
	if offsets.DataLengthFlash != 1024+FXPageSize {
		t.Fatalf("Expected flash datalength %d, got %d", 1024+FXPageSize, offsets.DataLengthFlash)
	}
	expectedDataLoc := offsets.SaveStart - offsets.DataLengthFlash
	if offsets.DataStart != expectedDataLoc {
		t.Fatalf("Expected data location %d, got %d", expectedDataLoc, offsets.DataStart)
	}

	// now we compare the fx data generated against a known good fxdata for the same
	// set of... well, data.
	fxoldgen, err := os.ReadFile(fileTestPath("fxdata.bin"))
	if err != nil {
		t.Fatalf("Couldn't read old fxdata: %s", err)
	}

	bbin := bin.Bytes()
	if !bytes.Equal(fxoldgen, bbin) {
		err = os.WriteFile("fxdata_test_error.bin", bbin, 0660)
		if err != nil {
			t.Fatalf("Couldn't write error file: %s", err)
		}
		difpos := 0
		for difpos = range min(len(fxoldgen), len(bbin)) {
			if fxoldgen[difpos] != bbin[difpos] {
				break
			}
		}
		t.Fatalf("Generated fxdata not the same at index %d! old length %d vs new %d", difpos, len(fxoldgen), len(bbin))
	}

	headerstr := header.String()
	expected := []string{
		"#pragma once",
		"FX_DATA_PAGE",
		"FX_DATA_BYTES",
		"FX_SAVE_PAGE",
		"FX_SAVE_BYTES",
		"FX::begin(FX_DATA_PAGE, FX_DATA_SAVE)",
		"uint24_t spritesheet",
		"spritesheetFrames",
		"spritesheetWidth",
		"spritesheetHeight",
	}
	for _, exp := range expected {
		if !strings.Contains(headerstr, exp) {
			t.Fatalf("Didn't write '%s' in real1 header. Header:\n%s", exp, headerstr)
		}
	}
}