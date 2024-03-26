package arduboy

import (
	"bytes"
	"testing"
)

func TestBinToHexTransparency(t *testing.T) {
	for i := 0; i < 100; i++ {
		b := make([]byte, 20+i*11)
		var w bytes.Buffer
		err := BinToHex(b, &w)
		if err != nil {
			t.Errorf("Error converting bin to hex: %s", err)
		}
		b2, err := HexToBin(&w)
		if err != nil {
			t.Errorf("Error converting hex back to bin: %s", err)
		}
		if !bytes.Equal(b, b2) {
			t.Errorf("BinToHex/HexToBin not transparent!")
		}
	}
}
