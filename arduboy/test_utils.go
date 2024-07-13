package arduboy

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	CartBuilderFolder = "cart_build"
)

func testPath() string {
	return filepath.Join("..", "testfiles")
}

func fileTestPath(filename string) string {
	return filepath.Join(testPath(), filename)
}

func fileHelperPath(filename string) string {
	return filepath.Join("..", "helpers", filename)
}

func newRandomFilepath(filename string) (string, error) {
	err := os.MkdirAll("ignore", 0770)
	if err != nil {
		return "", err
	}
	filename = time.Now().Format("20060102030405") + "_" + filename
	return filepath.Abs(filepath.Join("ignore", filename))
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
