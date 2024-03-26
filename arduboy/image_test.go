package arduboy

import (
	"bytes"
	"crypto/rand"

	"testing"
)

func TestRawToPaletted_Transparent(t *testing.T) {
	raw := make([]byte, ScreenBytes)

	for i := 0; i < 100; i++ {
		_, err := rand.Read(raw)
		if err != nil {
			t.Errorf("Error generating random bytes! %s", err)
		}
		gray, err := RawToPaletted(raw)
		if err != nil {
			t.Errorf("Error generating paletted! %s", err)
		}
		raw2, err := PalettedToRaw(gray)
		if err != nil {
			t.Errorf("Error generating raw from paletted! %s", err)
		}
		if !bytes.Equal(raw, raw2) {
			t.Errorf("Paletted not transparent!")
		}
	}
}

func imageToRawTransparent(t *testing.T, format string) {
	raw := make([]byte, ScreenBytes)
	for i := 0; i < 100; i++ {
		_, err := rand.Read(raw)
		if err != nil {
			t.Errorf("Error generating random bytes! %s", err)
		}
		gray, err := RawToPaletted(raw)
		if err != nil {
			t.Errorf("Error generating paletted! %s", err)
		}
		img, err := PalettedToImageBW(gray, format)
		if err != nil {
			t.Errorf("Error generating %s! %s", format, err)
		}
		paletted, err := ImageToPaletted(bytes.NewReader(img), 100)
		if err != nil {
			t.Errorf("Error generating paletted from %s! %s", format, err)
		}
		raw2, err := PalettedToRaw(paletted)
		if err != nil {
			t.Errorf("Error generating raw from paletted! %s", err)
		}
		if !bytes.Equal(raw, raw2) {
			t.Errorf("%s not transparent!", format)
		}
	}
}

func TestRawToImage_Gif_Transparent(t *testing.T) {
	imageToRawTransparent(t, "gif")
}

func TestRawToImage_Png_Transparent(t *testing.T) {
	imageToRawTransparent(t, "png")
}

func TestRawToPaletted_IncorrectSize(t *testing.T) {
	raw := make([]byte, ScreenBytes-1)
	_, err := RawToPaletted(raw)
	if err == nil {
		t.Error("Didn't throw error on too-small raw!")
	}
	raw = make([]byte, ScreenBytes+1)
	_, err = RawToPaletted(raw)
	if err == nil {
		t.Error("Didn't throw error on too-large raw!")
	}
}

func TestGrayscaleToRaw_IncorrectSize(t *testing.T) {
	raw := make([]byte, ScreenWidth*ScreenHeight-1)
	_, err := PalettedToRaw(raw)
	if err == nil {
		t.Error("Didn't throw error on too-small grayscale!")
	}
	raw = make([]byte, ScreenWidth*ScreenHeight+1)
	_, err = PalettedToRaw(raw)
	if err == nil {
		t.Error("Didn't throw error on too-large grayscale!")
	}
}
