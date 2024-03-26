package arduboy

import (
	"bytes"
	"crypto/rand"
	"image"
	"image/png"

	"testing"
)

func TestRawToPaletted_Transparent(t *testing.T) {
	raw := make([]byte, ScreenBytes)

	for i := 0; i < 100; i++ {
		_, err := rand.Read(raw)
		if err != nil {
			t.Errorf("Error generating random bytes! %s", err)
		}
		gray, err := RawToPalettedTitle(raw)
		if err != nil {
			t.Errorf("Error generating paletted! %s", err)
		}
		raw2, err := PalettedToRawTitle(gray)
		if err != nil {
			t.Errorf("Error generating raw from paletted! %s", err)
		}
		if !bytes.Equal(raw, raw2) {
			t.Errorf("Paletted not transparent!")
		}
	}
}

func randomImage(raw []byte, format string, t *testing.T) []byte {
	_, err := rand.Read(raw)
	if err != nil {
		t.Errorf("Error generating random bytes! %s", err)
	}
	p, err := RawToPalettedTitle(raw)
	if err != nil {
		t.Errorf("Error generating paletted! %s", err)
	}
	img, err := PalettedToImageTitleBW(p, format)
	if err != nil {
		t.Errorf("Error generating %s! %s", format, err)
	}
	return img
}

func imageToRawTransparent(t *testing.T, format string) {
	raw := make([]byte, ScreenBytes)
	for i := 0; i < 100; i++ {
		img := randomImage(raw, format, t)
		paletted, err := ImageToPalettedTitle(bytes.NewReader(img), 100)
		if err != nil {
			t.Errorf("Error generating paletted from %s! %s", format, err)
		}
		raw2, err := PalettedToRawTitle(paletted)
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
	_, err := RawToPalettedTitle(raw)
	if err == nil {
		t.Error("Didn't throw error on too-small raw!")
	}
	raw = make([]byte, ScreenBytes+1)
	_, err = RawToPalettedTitle(raw)
	if err == nil {
		t.Error("Didn't throw error on too-large raw!")
	}
}

func TestGrayscaleToRaw_IncorrectSize(t *testing.T) {
	raw := make([]byte, ScreenWidth*ScreenHeight-1)
	_, err := PalettedToRawTitle(raw)
	if err == nil {
		t.Error("Didn't throw error on too-small grayscale!")
	}
	raw = make([]byte, ScreenWidth*ScreenHeight+1)
	_, err = PalettedToRawTitle(raw)
	if err == nil {
		t.Error("Didn't throw error on too-large grayscale!")
	}
}

func checkComputed(computed *TileConfigComputed, tiles []*image.NRGBA, t *testing.T, hf int, vf int, sw int, sh int) {
	if computed.HFrames != hf {
		t.Errorf("Expected %d HFrames, got %d!", hf, computed.HFrames)
	}
	if computed.VFrames != vf {
		t.Errorf("Expected 1 VFrames, got %d!", computed.VFrames)
	}
	if computed.SpriteWidth != sw {
		t.Errorf("Expected %d width, got %d!", sw, computed.SpriteWidth)
	}
	if computed.SpriteHeight != ScreenHeight {
		t.Errorf("Expected %d height, got %d!", sh, computed.SpriteHeight)
	}
	if len(tiles) != hf*vf {
		t.Errorf("Expected %d tiles, got %d!", hf*vf, len(tiles))
	}
}

func TestSplitImageToTiles_SingleImage(t *testing.T) {
	raw := make([]byte, ScreenBytes)
	config := TileConfig{
		// All defaults should use the entire screen, no spacing
	}
	for i := 0; i < 10; i++ {
		img := randomImage(raw, "gif", t)
		tiles, computed, err := SplitImageToTiles(bytes.NewReader(img), &config)
		if err != nil {
			t.Errorf("Error splitting singular image! %s", err)
		}
		checkComputed(computed, tiles, t, 1, 1, ScreenWidth, ScreenHeight)
		var buf bytes.Buffer
		err = png.Encode(&buf, tiles[0])
		if err != nil {
			t.Errorf("Error converting nrgba back to png! %s", err)
		}
		paletted2, err := ImageToPalettedTitle(&buf, 100)
		if err != nil {
			t.Errorf("Error converting tile back to paletted! %s", err)
		}
		raw2, err := PalettedToRawTitle(paletted2)
		if err != nil {
			t.Errorf("Error converting paletted tile back to raw! %s", err)
		}
		if !bytes.Equal(raw, raw2) {
			t.Errorf("Single tile not transparent!")
		}
	}
}
