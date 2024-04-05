package arduboy

import (
	"bytes"
	"fmt"
	//"os"
	"testing"
)

func TestParseFxData_FromFiles(t *testing.T) {
	// Create a basic configuration
	config := FxData{
		Data: make(map[string]*FxDataField),
		Save: make(map[string]*FxDataField),
	}

	// Now, add the spritesheet as data
	config.Data["spritesheet"] = &FxDataField{
		Data:   fileTestPath("spritesheet.png"),
		Format: "image",
		Image: &FxDataImageConfig{
			Width:  16,
			Height: 16,
		},
	}

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

	// This is the exact length of the uneven.bin (perhaps programmatically read this?)
	if offsets.SaveLength != 1031 {
		t.Fatalf("Expeted savelength 1031, got %d", offsets.SaveLength)
	}
	if offsets.SaveLengthFlash != FxSaveAlignment {
		t.Fatalf("Expeted flash savelength %d, got %d", FxSaveAlignment, offsets.SaveLengthFlash)
	}
	expectedSaveLoc := FxDevExpectedFlashCapacity - FxSaveAlignment
	if offsets.SaveStart != expectedSaveLoc {
		t.Fatalf("Expeted save location %d, got %d", expectedSaveLoc, offsets.SaveStart)
	}

	fmt.Printf("Header:\n%s", string(header.Bytes()))
}
