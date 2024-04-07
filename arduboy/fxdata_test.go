package arduboy

import (
	"bytes"
	//"fmt"
	"os"
	"testing"
)

func TestParseFxData_FromFiles(t *testing.T) {
	// Create a basic configuration
	config := FxData{
		Data:     make(map[string]*FxDataField),
		Save:     make(map[string]*FxDataField),
		KeyOrder: []string{"spritesheet", "myhex", "mybase64", "mystring"},
	}

	// Now, add the spritesheet as data
	config.Data["spritesheet"] = &FxDataField{
		Data:   fileTestPath("spritesheet.png"),
		Format: "image",
		Image: &FxDataImageConfig{
			// NOTE: unfortunately, can't use width and height, because original
			// fxdata requires special filenames (blegh)
			//Width:   16,
			//Height:  16,
			UseMask:        true,
			Threshold:      100,
			AlphaThreshold: 10,
		},
	}

	// And add some hex. this should be 17 bytes
	config.Data["myhex"] = &FxDataField{
		Data:   "000102030405060708090A0B0C0D0E0F10",
		Format: "hex",
	}

	// And some base64 of "Hello world!". 12 bytes (no null terminator)
	config.Data["mybase64"] = &FxDataField{
		Data:   "SGVsbG8gd29ybGQh",
		Format: "base64",
	}

	// And finally an ACTUAL string. 41 bytes + 1 (null terminator)
	config.Data["mystring"] = &FxDataField{
		// NOTE: I wanted to use ' and " and \ in the test, but the python fxdata parser
		// doesn't work with those...
		Data:   "owo uwu !@#$%^&*()-_[]{}|;:?/.><,+=`~Z188",
		Format: "string",
	}

	// Total bytes are 1028 + 17 + 12 + 42 = 1099

	// And add some raw data as initial save.
	config.Save["uneven"] = &FxDataField{
		Data:   fileTestPath("uneven.bin"),
		Format: "file",
	}

	// Make some buffers to hold the data
	var header bytes.Buffer
	var bin bytes.Buffer

	// Call the function
	offsets, err := ParseFxData(&config, &header, &bin)
	if err != nil {
		t.Fatalf("Error returned from ParseFxData: %s", err)
	}

	//fmt.Printf("Header:\n%s", string(header.Bytes()))

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

}
