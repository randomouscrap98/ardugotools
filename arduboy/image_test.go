package arduboy

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"image"
	"image/png"
	"os"
	"path"

	"testing"
)

func TestRawToPaletted_Transparent(t *testing.T) {
	raw := make([]byte, ScreenBytes)

	for i := 0; i < 100; i++ {
		_, err := rand.Read(raw)
		if err != nil {
			t.Fatalf("Error generating random bytes! %s", err)
		}
		gray, err := RawToPalettedTitle(raw)
		if err != nil {
			t.Fatalf("Error generating paletted! %s", err)
		}
		raw2, err := PalettedToRawTitle(gray)
		if err != nil {
			t.Fatalf("Error generating raw from paletted! %s", err)
		}
		if !bytes.Equal(raw, raw2) {
			t.Fatalf("Paletted not transparent!")
		}
	}
}

func randomImage(raw []byte, format string, t *testing.T) []byte {
	_, err := rand.Read(raw)
	if err != nil {
		t.Fatalf("Error generating random bytes! %s", err)
	}
	p, err := RawToPalettedTitle(raw)
	if err != nil {
		t.Fatalf("Error generating paletted! %s", err)
	}
	img, err := PalettedToImageTitleBW(p, format)
	if err != nil {
		t.Fatalf("Error generating %s! %s", format, err)
	}
	return img
}

func imageToRawTransparent(t *testing.T, format string) {
	raw := make([]byte, ScreenBytes)
	for i := 0; i < 100; i++ {
		img := randomImage(raw, format, t)
		paletted, err := RawImageToPalettedTitle(bytes.NewReader(img), 100)
		if err != nil {
			t.Fatalf("Error generating paletted from %s! %s", format, err)
		}
		raw2, err := PalettedToRawTitle(paletted)
		if err != nil {
			t.Fatalf("Error generating raw from paletted! %s", err)
		}
		if !bytes.Equal(raw, raw2) {
			t.Fatalf("%s not transparent!", format)
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
		t.Fatalf("Expected %d HFrames, got %d!", hf, computed.HFrames)
	}
	if computed.VFrames != vf {
		t.Fatalf("Expected 1 VFrames, got %d!", computed.VFrames)
	}
	if computed.SpriteWidth != sw {
		t.Fatalf("Expected %d width, got %d!", sw, computed.SpriteWidth)
	}
	if computed.SpriteHeight != ScreenHeight {
		t.Fatalf("Expected %d height, got %d!", sh, computed.SpriteHeight)
	}
	if len(tiles) != hf*vf {
		t.Fatalf("Expected %d tiles, got %d!", hf*vf, len(tiles))
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
			t.Fatalf("Error splitting singular image! %s", err)
		}
		checkComputed(computed, tiles, t, 1, 1, ScreenWidth, ScreenHeight)
		var buf bytes.Buffer
		err = png.Encode(&buf, tiles[0])
		if err != nil {
			t.Fatalf("Error converting nrgba back to png! %s", err)
		}
		paletted2, err := RawImageToPalettedTitle(&buf, 100)
		if err != nil {
			t.Fatalf("Error converting tile back to paletted! %s", err)
		}
		raw2, err := PalettedToRawTitle(paletted2)
		if err != nil {
			t.Fatalf("Error converting paletted tile back to raw! %s", err)
		}
		if !bytes.Equal(raw, raw2) {
			t.Fatalf("Single tile not transparent!")
		}
	}
}

func tileTestPath(filename string) string {
	return path.Join("..", "testfiles", "tiles", filename)
}

func TestSplitImageToTiles_TestFile_NoSpacing(t *testing.T) {
	// Get the basic file
	ssraw, err := os.Open(tileTestPath("spritesheet_test.png"))
	if err != nil {
		t.Fatalf("Couldn't open test spritesheet: %s", err)
	}
	defer ssraw.Close()
	config := TileConfig{
		Width:  16,
		Height: 16,
	}
	tiles, computed, err := SplitImageToTiles(ssraw, &config)
	if err != nil {
		t.Fatalf("Couldn't split image: %s", err)
	}
	if computed.SpriteWidth != 16 {
		t.Fatalf("Sprites not 16 width: %d", computed.SpriteWidth)
	}
	if computed.SpriteHeight != 16 {
		t.Fatalf("Sprites not 16 height: %d", computed.SpriteHeight)
	}
	if computed.HFrames != 4 {
		t.Fatalf("HFrames not 4: %d", computed.HFrames)
	}
	if computed.VFrames != 3 {
		t.Fatalf("VFrames not 4: %d", computed.VFrames)
	}
	if len(tiles) != 12 {
		t.Fatalf("Not 12 tiles: %d", len(tiles))
	}
	for i, tile := range tiles {
		// have to go load the original picture
		tileraw, err := os.Open(tileTestPath(fmt.Sprintf("%d.png", i)))
		if err != nil {
			t.Fatalf("Couldn't open test spritesheet: %s", err)
		}
		defer tileraw.Close()
		realtilepaletted, _, _, err := RawImageToPaletted(tileraw, 100)
		if err != nil {
			t.Fatalf("Couldn't get paletted from reference tile: %s", err)
		}
		testtilepaletted, _, _ := ImageToPaletted(tile, 100)
		if !bytes.Equal(realtilepaletted, testtilepaletted) {
			t.Fatalf("Tile %d not transparent!", i)
		}
	}
}
