package arduboy

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunLuaFlashcartGenerator_Arguments(t *testing.T) {
	script := `
a, b, c = arguments()
log(a, b, c)
  `

	arguments := []string{"what", "how", "this -- is == weird"}

	logs, err := RunLuaFlashcartGenerator(script, arguments, "")
	if err != nil {
		t.Fatalf("Error running basic flashcart generator: %s", err)
	}

	expected := "what\thow\tthis -- is == weird\n"
	if logs != expected {
		t.Fatalf("Expected logs '%s', got '%s'", expected, logs)
	}
}

func TestRunLuaFlashcartGenerator_ReadBasic(t *testing.T) {
	script := `
slots = parse_flashcart("minicart.bin")
log("Slot count: " .. #slots)
for i,v in ipairs(slots) do
  assert(#v.image == 1024)
  if v.is_category then
    log(v.title .. " - Category")
  else
    log(v.title .. " " .. v.version .. " - " .. v.developer)
  end
end
  `

	//arguments := []string{"what", "how", "this -- is == weird"}
	logs, err := RunLuaFlashcartGenerator(script, nil, testPath())
	if err != nil {
		t.Fatalf("Error running basic read flashcart generator: %s", err)
	}

	lines := strings.Split(logs, "\n")

	expected := []string{
		"Slot count: 13",
		"Bootloader - Category",
		"Action - Category",
		"Hopper 1.0 - Obono",
		"Lasers 1.0 - Obono",
		"Chri-Bocchi Cat 1.0 - Obono",
		"Bangi 1.0 - Igvina",
		"Helii 1.0 - BHSPitMonkey",
		"Choplifter 1.1.1 - Press Play On Tape",
		"Adventure - Category",
		"Catacombs Of The Damned 1.0 - jhhoward",
		"Virus LQP-79 1.0 - Team ARG",
		"Glove 1.0 - fuopy",
		"Mazogs 1.0 - Brian",
	}

	for i := range expected {
		if lines[i] != expected[i] {
			t.Fatalf("Expected at [%d] '%s', got '%s'", i, expected[i], lines[i])
		}
	}
}

func TestRunLuaFlashcartGenerator_SimpleTransparent(t *testing.T) {
	script := `
a = arguments()
slots = parse_flashcart("minicart.bin", true)
newcart = new_flashcart(a)
for i,v in ipairs(slots) do
  newcart.write_slot(v)
end
  `
	testpath, err := newRandomFilepath("transparent.bin")
	if err != nil {
		t.Fatalf("Couldn't get path to test file: %s", err)
	}

	arguments := []string{testpath}
	_, err = RunLuaFlashcartGenerator(script, arguments, testPath())
	if err != nil {
		t.Fatalf("Couldn't run flashcart generator: %s", err)
	}

	_, err = os.Stat(testpath)
	if err != nil {
		t.Fatalf("Couldn't stat test file %s: %s", testpath, err)
	}

	// Compare the two files
	minibin, err := os.ReadFile(fileTestPath("minicart.bin"))
	if err != nil {
		t.Fatalf("Couldn't read minicart.bin: %s", err)
	}
	testbin, err := os.ReadFile(testpath)
	if err != nil {
		t.Fatalf("Couldn't read %s: %s", testpath, err)
	}

	if !bytes.Equal(minibin, testbin) {
		t.Fatalf("Written flashcart not equivalent!")
	}
}

// This mostly tests the title image converter
func TestRunLuaFlashcartGenerator_CategoriesOnly(t *testing.T) {
	script := `
a, t1, t2 = arguments()
newcart = new_flashcart(a)
newcart.write_slot({
  title = "Bootloader",
  info = "That is a legacy computer wowee",
  image = title_image(t1),
})
newcart.write_slot({
  title = "Adventure",
  info = "Go on an adventure! You know you want to! (:)",
  -- Might as well test to make sure these values don't get written
  version = "NO NO NO",
  developer = "PHANTOM",
  image = title_image(t2),
})
  `
	testpath, err := newRandomFilepath("onlycategories.bin")
	if err != nil {
		t.Fatalf("Couldn't get path to test file: %s", err)
	}
	title1 := fileTestPath(filepath.Join(CartBuilderFolder, "bootloader.png"))
	title2 := fileTestPath(filepath.Join(CartBuilderFolder, "games.png"))

	arguments := []string{testpath, title1, title2}
	_, err = RunLuaFlashcartGenerator(script, arguments, testPath())
	if err != nil {
		t.Fatalf("Couldn't run flashcart generator: %s", err)
	}

	_, err = os.Stat(testpath)
	if err != nil {
		t.Fatalf("Couldn't stat test file %s: %s", testpath, err)
	}

	// Compare the two files
	expectedbin, err := os.ReadFile(fileTestPath("onlycategories.bin"))
	if err != nil {
		t.Fatalf("Couldn't read onlycategories.bin: %s", err)
	}
	testbin, err := os.ReadFile(testpath)
	if err != nil {
		t.Fatalf("Couldn't read %s: %s", testpath, err)
	}

	if !bytes.Equal(expectedbin, testbin) {
		t.Fatalf("Written flashcart not equivalent! %d bytes vs %d", len(testbin), len(expectedbin))
	}
}
