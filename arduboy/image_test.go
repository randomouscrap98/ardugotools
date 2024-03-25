package arduboy

import (
	"bytes"
	"crypto/rand"

	"testing"
)

func TestRawToGrayscale_Transparent(t *testing.T) {
	raw := make([]byte, ScreenBytes)

	for i := 0; i < 100; i++ {
		_, err := rand.Read(raw)
		if err != nil {
			t.Errorf("Error generating random bytes! %s", err)
		}
		gray, err := RawToGrayscale(raw, 0, 255)
		if err != nil {
			t.Errorf("Error generating grayscale! %s", err)
		}
		raw2, err := GrayscaleToRaw(gray, 100)
		if err != nil {
			t.Errorf("Error generating raw from grayscale! %s", err)
		}
		if !bytes.Equal(raw, raw2) {
			t.Errorf("Not transparent!")
		}
	}
}

func TestRawToGrayscale_ImageToRaw_Transparent(t *testing.T) {
	raw := make([]byte, ScreenBytes)
	for i := 0; i < 100; i++ {
		_, err := rand.Read(raw)
		if err != nil {
			t.Errorf("Error generating random bytes! %s", err)
		}
		gray, err := RawToGrayscale(raw, 0, 255)
		if err != nil {
			t.Errorf("Error generating grayscale! %s", err)
		}
		graypng, err := GrayscaleToPng(gray)
		if err != nil {
			t.Errorf("Error generating png! %s", err)
		}
		raw2, err := ImageToRaw(bytes.NewReader(graypng), 100)
		if err != nil {
			t.Errorf("Error generating raw from png! %s", err)
		}
		if !bytes.Equal(raw, raw2) {
			t.Errorf("Not transparent!")
		}
	}
}

func TestRawToGrayscale_IncorrectSize(t *testing.T) {
	raw := make([]byte, ScreenBytes-1)
	_, err := RawToGrayscale(raw, 0, 255)
	if err == nil {
		t.Error("Didn't throw error on too-small raw!")
	}
	raw = make([]byte, ScreenBytes+1)
	_, err = RawToGrayscale(raw, 0, 255)
	if err == nil {
		t.Error("Didn't throw error on too-large raw!")
	}
}

func TestGrayscaleToRaw_IncorrectSize(t *testing.T) {
	raw := make([]byte, ScreenWidth*ScreenHeight-1)
	_, err := GrayscaleToRaw(raw, 100)
	if err == nil {
		t.Error("Didn't throw error on too-small grayscale!")
	}
	raw = make([]byte, ScreenWidth*ScreenHeight+1)
	_, err = GrayscaleToRaw(raw, 100)
	if err == nil {
		t.Error("Didn't throw error on too-large grayscale!")
	}
}
