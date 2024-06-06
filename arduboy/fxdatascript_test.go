package arduboy

import (
	"bytes"
	"testing"
)

func TestRunLuaFxGenerator_BasicSetup(t *testing.T) {
	script := "preamble()"

	var header bytes.Buffer
	var bin bytes.Buffer

	_, err := RunLuaFxGenerator(script, &header, &bin)
	if err != nil {
		t.Fatalf("Error running basic fx generator: %s", err)
	}
}
