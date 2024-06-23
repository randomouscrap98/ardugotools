package arduboy

import (
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
