package arduboy

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunLuaFxGenerator_Empty(t *testing.T) {
	script := "-- Nothing here!"

	var header bytes.Buffer
	var bin bytes.Buffer

	_, err := RunLuaFxGenerator(script, &header, &bin)
	if err != nil {
		t.Fatalf("Error running basic fx generator: %s", err)
	}

	headerstr := string(header.Bytes())

	expected := []string{
		"#pragma once",
		"FX_DATA_PAGE",
		"FX_DATA_BYTES",
	}
	for _, exp := range expected {
		if strings.Index(headerstr, exp) < 0 {
			t.Fatalf("Didn't write '%s' in empty header. Header:\n%s", exp, headerstr)
		}
	}
}

func TestRunLuaFxGenerator_SaveOnly(t *testing.T) {
	script := "begin_save()"

	var header bytes.Buffer
	var bin bytes.Buffer

	offsets, err := RunLuaFxGenerator(script, &header, &bin)
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

	headerstr := string(header.Bytes())

	expectedheaders := []string{
		"#pragma once",
		"FX_DATA_PAGE",
		"FX_DATA_BYTES",
		"FX_SAVE_PAGE",
		"FX_SAVE_BYTES",
	}
	for _, exp := range expectedheaders {
		if strings.Index(headerstr, exp) < 0 {
			t.Fatalf("Didn't write '%s' in empty header. Header:\n%s", exp, headerstr)
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

	offsets, err := RunLuaFxGenerator(script, &header, &bin)
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

	headerstr := string(header.Bytes())
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
		if strings.Index(headerstr, exp) < 0 {
			t.Fatalf("Didn't write '%s' in empty header. Header:\n%s", exp, headerstr)
		}
	}
}
