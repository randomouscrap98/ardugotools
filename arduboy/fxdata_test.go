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

// Uint24 is a special thing
func TestRunLuaFxGenerator_Uint24(t *testing.T) {
	script := `
pointers = {}
table.insert(pointers, field("data1"))
write("it's something")             -- 14 bytes
myaddr = field("data2")
assert(myaddr == 14, "address 14, was " .. myaddr)
table.insert(pointers, myaddr)
write("it's REALLY something")      -- 21 bytes
table.insert(pointers, field("data3"))
write("it's over...")               -- 12 bytes
field("pointers")
written = write(bytes(pointers, "uint24"))    -- 9 bytes
assert(#pointers == 3, "pointers 3, was " .. #pointers)
assert(written == 9, "written 9, was " .. written)
`

	var header bytes.Buffer
	var bin bytes.Buffer

	offsets, err := RunLuaFxGenerator(script, &header, &bin, "")
	if err != nil {
		t.Fatalf("Error running uint24 fx generator: %s", err)
	}

	expectedDataLength := 56
	if offsets.DataLength != expectedDataLength {
		t.Fatalf("Expected DataLength=%d, got %d", expectedDataLength, offsets.DataLength)
	}
	if offsets.DataLengthFlash != FXPageSize {
		t.Fatalf("Expected DataLengthFlash=%d, got %d", FXPageSize, offsets.DataLengthFlash)
	}

	headerstr := header.String()
	bbs := bin.Bytes()

	if len(bbs) != FXPageSize {
		t.Fatalf("Expected %d bytes, got %d", FXPageSize, len(bbs))
	}

	expectedheaders := []string{
		"constexpr uint24_t data1 = 0x000000;",
		"constexpr uint24_t data2 = 0x00000E;",
		"constexpr uint24_t data3 = 0x000023;",
		"constexpr uint24_t pointers = 0x00002F;",
	}
	for _, exp := range expectedheaders {
		if !strings.Contains(headerstr, exp) {
			t.Fatalf("Didn't write '%s' in header. Header:\n%s", exp, headerstr)
		}
	}

	// Now make sure the data even makes sense
	expected := []byte{0, 0, 0, 14, 0, 0, 35, 0, 0}
	// There's padding, be careful!!
	actualbbs := bbs[expectedDataLength-len(expected) : expectedDataLength]
	if len(actualbbs) != 9 {
		t.Fatalf("Expected 9 bbs bytes, got %d", len(actualbbs))
	}
	if !bytes.Equal(actualbbs, expected[:]) {
		t.Fatalf("Unexpected pointers in uint24 area: %v", actualbbs)
	}
}

func TestRunLuaFxGenerator_Raycast(t *testing.T) {
	script := `
-- This first one tests the normal parameter passing
sprites,frames,width,height = image("spritesheet.png", 32, 32, 0, true, 100, 10, true)
written = raycast_helper("spritesheet", true, {
	["32"] = sprites,
	["16"] = image_resize(sprites, width, height, 16, 16),
	["8"] = image_resize(sprites, width, height, 8, 8),
	["4"] = image_resize(sprites, width, height, 4, 4),
})
header("// Raycast bytes written: " .. written .. "\n")
-- And this one tests the table
sproots,frames,width,height = image({
	filename = "spritesheet.png",
	width = 32,
	height = 32,
	usemask = true,
	rawtiles = true,
})
written = raycast_helper("sprootsheet", false, {
	["32"] = sproots,
	["16"] = image_resize(sproots, width, height, 16, 16),
	["8"] = image_resize(sproots, width, height, 8, 8),
	["4"] = image_resize(sproots, width, height, 4, 4),
})
header("// Raycast bytes written2: " .. written .. "\n")
`

	var header bytes.Buffer
	var bin bytes.Buffer

	offsets, err := RunLuaFxGenerator(script, &header, &bin, testPath())
	if err != nil {
		t.Fatalf("Error running basic fx generator: %s", err)
	}

	headerstr := header.String()
	fxd := bin.Bytes()

	var tilesize int
	//var maxmipmap int
	//var mipmapbytes int
	tsindex := strings.Index(headerstr, "Raycast frame bytes:")
	fmt.Sscanf(headerstr[tsindex:], "Raycast frame bytes: %d", &tilesize)

	expectedDataLength := tilesize * 4 * 3
	if offsets.DataLength != expectedDataLength {
		t.Fatalf("Expected DataLength=%d, got %d", expectedDataLength, offsets.DataLength)
	}

	expectedheaders := []string{
		"constexpr uint24_t spritesheet",
		"constexpr uint24_t sprootsheet",
		"constexpr uint24_t spritesheetMask",
		"spritesheetFrames = 4",
		"sprootsheetFrames = 4",
		fmt.Sprintf("Raycast bytes written: %d", tilesize*4*2),
		fmt.Sprintf("Raycast bytes written2: %d", tilesize*4),
	}
	for _, exp := range expectedheaders {
		if !strings.Contains(headerstr, exp) {
			t.Fatalf("Didn't write '%s' in header. Header:\n%s", exp, headerstr)
		}
	}
	if strings.Contains(headerstr, "sprootsheetMask") {
		t.Fatalf("Unexpected write '%s' in header. Header:\n%s", "sprootsheetMask", headerstr)
	}

	rawImg, err := os.Open(fileTestPath("spritesheet.png"))
	if err != nil {
		t.Fatalf("Can't open spritesheet.png for comparison: %s", err)
	}
	defer rawImg.Close()

	config := TileConfig{
		Width:  32,
		Height: 32,
	}
	tiles, _, err := SplitImageToTiles(rawImg, &config)
	if err != nil {
		t.Fatalf("Can't open spritesheet.png for comparison: %s", err)
	}

	//ptiles := make([][]byte, len(tiles))
	for i, tile := range tiles {
		ptile, _, _ := ImageToPaletted(tile, 100, 10)
		// Check if the 32 mipmap is what we expect
		for x := 0; x < config.Width; x++ {
			var fulltile uint32
			var fullmask uint32
			for b := 0; b < 4; b++ {
				fulltile |= uint32(fxd[i*tilesize+x*4+b]) << (8 * b)
				fullmask |= uint32(fxd[(i+4)*tilesize+x*4+b]) << (8 * b)
			}
			//log.Printf("xt: %08x xm: %08x", fulltile, fullmask)
			for y := 0; y < config.Height; y++ {
				expected := ptile[x+y*config.Width]
				actual := byte(1 & (fulltile >> y))
				actualmask := byte(1 & (fullmask >> y))
				if expected == 2 && (actualmask != 0 || actual != 0) {
					t.Fatalf("Error in tile %d at (%d,%d): Expected transparency, got %d|%d", i, x, y, actual, actualmask)
				}
				if expected == 1 && (actualmask != 1 || actual != 1) {
					t.Fatalf("Error in tile %d at (%d,%d): Expected white, got %d|%d", i, x, y, actual, actualmask)
				}
				if expected == 0 && (actualmask != 1 || actual != 0) {
					t.Fatalf("Error in tile %d at (%d,%d): Expected black, got %d|%d", i, x, y, actual, actualmask)
				}
			}
		}
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
