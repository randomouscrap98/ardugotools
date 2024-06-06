package arduboy

import (
	"bytes"
	"strings"
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

	headerstr := string(header.Bytes())

	if strings.Index(headerstr, "#pragma once") < 0 {
		t.Fatalf("Didn't write pragma once header when asked. Header:\n%s", headerstr)
	}
}
