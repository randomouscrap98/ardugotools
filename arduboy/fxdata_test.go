package arduboy

import (
	"bytes"
	"os"
	"testing"
)


func newFxDataOutput(folder string) (os.File, error) {
  // Just use file backing because ugh
  os.MkdirAll(folder, os.ModePerm)


	output := FxDataOutput{
		Header: hbuf,
		Data:   dbuf,
		Save:   sbuf,
		Dev:    devbuf,
	}
}

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
		Image:  &FxDataImageConfig{},
	}

	// And add some raw data as initial save.
	config.Save["uneven"] = &FxDataField{
		Data:   fileTestPath("uneven.bin"),
		Format: "file",
	}

	// Make a bunch of buffers to hold the data
	hbuf := bytes.//NewBuffer(nil)
	dbuf := bytes.NewBuffer(nil)
	sbuf := bytes.NewBuffer(nil)
	devbuf := bytes.NewBuffer(nil)

	output := FxDataOutput{
		Header: hbuf,
		Data:   dbuf,
		Save:   sbuf,
		Dev:    devbuf,
	}

}
