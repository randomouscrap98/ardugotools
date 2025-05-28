package arduboy

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
		t.Fatalf("Error running basic flashcart generator for argument testing: %s", err)
	}

	expected := "what\thow\tthis -- is == weird\n"
	if logs != expected {
		t.Fatalf("Expected logs '%s', got '%s'", expected, logs)
	}
}

func TestRunLuaFlashcartGenerator_Toml(t *testing.T) {
	script := `
t = toml("[Header]\nmyvalue=55\nmystring=\"yes\"\n[Header.Extra]\nmyarray=[68.75]")
log(t.Header.myvalue, t.Header.mystring, t.Header.Extra.myarray[1])
  `

	logs, err := RunLuaFlashcartGenerator(script, nil, "")
	if err != nil {
		t.Fatalf("Error running basic flashcart generator for toml testing: %s", err)
	}

	expected := "55\tyes\t68.75\n"
	if logs != expected {
		t.Fatalf("Expected logs '%s', got '%s'", expected, logs)
	}
}

// This function is available in all lua runtimes but I test it here
// because it's where we expect to use it
func TestRunLuaFlashcartGenerator_ListDir(t *testing.T) {
	script := `
mydir = arguments()
results = listdir(mydir)
for _, dinfo in ipairs(results) do
  log(dinfo.name .. "#" .. tostring(dinfo.is_directory) .. "#" .. dinfo.path)
end
  `

	arguments := []string{testPath()}

	logs, err := RunLuaFlashcartGenerator(script, arguments, "")
	if err != nil {
		t.Fatalf("Error running basic flashcart generator: %s", err)
	}

	lines := strings.Split(logs, "\n")
	expected := []string{
		`cart_build#true#.+?testfiles[/\\]cart_build`,
		`flashcart\.lua#false#.+?testfiles[/\\]flashcart\.lua`,
		`tiles#true#.+?testfiles[/\\]tiles`,
		`uneven\.bin#false#.+?testfiles[/\\]uneven\.bin`,
	}

	for _, exp := range expected {
		found := false
		for _, line := range lines {
			found, err = regexp.Match(exp, []byte(line))
			if err != nil {
				t.Fatalf("Error matching regex: %s", err)
			}
			if found {
				break
			}
		}
		if !found {
			fmt.Print(logs)
			t.Fatalf("Couldn't find expected line: %s", exp)
		}
	}
}

func TestRunLuaFlashcartGenerator_ReadBasic(t *testing.T) {
	script := `
slots = parse_flashcart("minicart.bin")
log("Slot count: " .. #slots)
for i,v in ipairs(slots) do
  assert(#v.image == 1024)
  if is_category(v) then
    log(v.title .. " - Category")
  else
    log(v.title .. " " .. v.version .. " - " .. v.developer)
  end
end
  `

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
	title2 := fileTestPath(filepath.Join(CartBuilderFolder, "title.png"))

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

func loadFullCart(name string, t *testing.T) []byte {
	testzip := fileTestPath("fullcarts.zip")
	archive, err := zip.OpenReader(testzip)
	if err != nil {
		t.Fatalf("Couldn't open fullcarts.zip: %s", err)
	}
	defer archive.Close()
	f, err := archive.Open(name)
	if err != nil {
		t.Fatalf("Couldn't find %s in fullcarts.zip: %s", name, err)
	}
	defer f.Close()
	result, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("Couldn't read %s in fullcarts.zip: %s", name, err)
	}
	return result
}

func TestRunLuaFlashcartGenerator_FullCart(t *testing.T) {
	script := `
a, t1, t2, t3, t4, p1, p2, p3 = arguments()
newcart = new_flashcart(a)
newcart.write_slot({
  title = "Bootloader",
  image = title_image(t1),
})
newcart.write_slot({
  title = "Games",
  image = title_image(t2),
})
-- This is TexasHoldEmFX. There's only one device inside, but we'll specify it 
-- anyway just in case. The correct image SHOULD be chosen, since there's only one
newcart.write_slot(package(p1, "ArduboyFX"))
-- This is microcity. the package is well-formed and there's only ONE device in it
newcart.write_slot(package(p2))
newcart.write_slot({
  title = "Horror",
  image = title_image(t3),
})
-- This is prince of arabia. There is NO image in this package, so we add our own
slot = package(p3)
slot.image = title_image(t4)
newcart.write_slot(slot)
  `
	testpath, err := newRandomFilepath("fulltest.bin")
	if err != nil {
		t.Fatalf("Couldn't get path to test file: %s", err)
	}

	titles := []string{"bootloader.png", "title.png", "horror.png", "PrinceOfArabia.V1.3.png"}
	packages := []string{"TexasHoldEmFX.arduboy", "MicroCity.arduboy", "PrinceOfArabia.V1.3.arduboy"}

	arguments := []string{testpath}

	for _, t := range titles {
		arguments = append(arguments, fileTestPath(filepath.Join(CartBuilderFolder, t)))
	}
	for _, p := range packages {
		arguments = append(arguments, fileTestPath(filepath.Join(CartBuilderFolder, p)))
	}

	_, err = RunLuaFlashcartGenerator(script, arguments, testPath())
	if err != nil {
		t.Fatalf("Couldn't run flashcart generator: %s", err)
	}

	_, err = os.Stat(testpath)
	if err != nil {
		t.Fatalf("Couldn't stat test file %s: %s", testpath, err)
	}

	// Compare the two files
	expectedbin := loadFullCart("cart_menu.bin", t)
	testbin, err := os.ReadFile(testpath)
	if err != nil {
		t.Fatalf("Couldn't read %s: %s", testpath, err)
	}

	if !bytes.Equal(expectedbin, testbin) {
		t.Fatalf("Written flashcart not equivalent! %d bytes vs %d", len(testbin), len(expectedbin))
	}
}

func TestRunLuaFlashcartGenerator_AddToCategory(t *testing.T) {
	script, err := os.ReadFile(fileHelperPath("addorupdate.lua"))
	if err != nil {
		t.Fatalf("Couldn't read lua script: %s", err)
	}
	basebin := loadFullCart("upsert_base.bin", t)
	gamepath := fileTestPath(filepath.Join(CartBuilderFolder, "3dMaze.arduboy"))
	basebinpath, err := newRandomFilepath("upsert_base.bin")
	if err != nil {
		t.Fatalf("Couldn't create random file to store base bin: %s", err)
	}
	err = os.WriteFile(basebinpath, basebin, 0600)
	if err != nil {
		t.Fatalf("Couldn't write file to store base bin: %s", err)
	}

	categories := []string{
		"Depression", "Anxiety", "Null", "Programming",
	}

	// This is the insert test: the game is new
	for i, category := range categories {
		catnum := i + 1
		thisbin := loadFullCart(fmt.Sprintf("upsert_cat%d.bin", catnum), t)
		// Insert into each of the 4 categories
		newbinpath, err := newRandomFilepath(fmt.Sprintf("upsert_test%d.bin", catnum))
		if err != nil {
			t.Fatalf("Couldn't create new file for test %d: %s", catnum, err)
		}
		arguments := []string{gamepath, "Arduboy,ArduboyFX", category, basebinpath, newbinpath}
		errout, err := RunLuaFlashcartGenerator(string(script), arguments, testPath())
		if err != nil {
			t.Fatalf("Couldn't run flashcart generator: %s. Log: \n%s", err, errout)
		}
		// Compare the two files
		testbin, err := os.ReadFile(newbinpath)
		if err != nil {
			t.Fatalf("Couldn't read %s: %s", newbinpath, err)
		}
		if !bytes.Equal(thisbin, testbin) {
			t.Fatalf("Written flashcart not equivalent! %d bytes vs %d", len(testbin), len(thisbin))
		}
	}

	// This is the update test
	gamepath = fileTestPath(filepath.Join(CartBuilderFolder, "OldMiner_Modded.arduboy"))
	thisbin := loadFullCart("upsert_updateminer.bin", t)
	newbinpath, err := newRandomFilepath("upsert_update.bin")
	if err != nil {
		t.Fatalf("Couldn't create new file for update test: %s", err)
	}
	arguments := []string{gamepath, "Arduboy,ArduboyFX", categories[1], basebinpath, newbinpath}
	errout, err := RunLuaFlashcartGenerator(string(script), arguments, testPath())
	if err != nil {
		t.Fatalf("Couldn't run flashcart generator: %s. Log: \n%s", err, errout)
	}
	// Compare the two files
	testbin, err := os.ReadFile(newbinpath)
	if err != nil {
		t.Fatalf("Couldn't read %s: %s", newbinpath, err)
	}
	if !bytes.Equal(thisbin, testbin) {
		t.Fatalf("Written flashcart not equivalent! %d bytes vs %d", len(testbin), len(thisbin))
	}
}

func TestRunLuaFlashcartGenerator_ApplySaves(t *testing.T) {
	script, err := os.ReadFile(fileHelperPath("applysaves.lua"))
	if err != nil {
		t.Fatalf("Couldn't read lua script: %s", err)
	}
	basebin := loadFullCart("fxsave_base.bin", t)
	basebinpath, err := newRandomFilepath("fxsave_base.bin")
	if err != nil {
		t.Fatalf("Couldn't create random file to store base bin: %s", err)
	}
	err = os.WriteFile(basebinpath, basebin, 0600)
	if err != nil {
		t.Fatalf("Couldn't write file to store base bin: %s", err)
	}
	newbin := loadFullCart("fxsave_new.bin", t)
	newbinpath, err := newRandomFilepath("fxsave_new.bin")
	if err != nil {
		t.Fatalf("Couldn't create random file to store new bin: %s", err)
	}
	err = os.WriteFile(newbinpath, newbin, 0600)
	if err != nil {
		t.Fatalf("Couldn't write file to store new bin: %s", err)
	}
	combinedbin := loadFullCart("fxsave_combined.bin", t)

	outbinpath, err := newRandomFilepath("fxsave_combined.bin")
	if err != nil {
		t.Fatalf("Couldn't create new file to store final bin: %s", err)
	}
	arguments := []string{basebinpath, newbinpath, outbinpath} //gamepath, "Arduboy,ArduboyFX", categories[1], basebinpath, newbinpath}
	errout, err := RunLuaFlashcartGenerator(string(script), arguments, testPath())
	if err != nil {
		t.Fatalf("Couldn't run flashcart generator: %s. Log: \n%s", err, errout)
	}
	// Compare the two files
	testbin, err := os.ReadFile(outbinpath)
	if err != nil {
		t.Fatalf("Couldn't read %s: %s", outbinpath, err)
	}
	if !bytes.Equal(combinedbin, testbin) {
		t.Fatalf("Written flashcart not equivalent! %d bytes vs %d", len(testbin), len(combinedbin))
	}
}

func TestRunLuaFlashcartGenerator_MakeCart(t *testing.T) {
	script, err := os.ReadFile(fileHelperPath("makecart.lua"))
	if err != nil {
		t.Fatalf("Couldn't read lua script: %s", err)
	}

	comparebin := loadFullCart("makecart.bin", t)

	outbinpath, err := newRandomFilepath("makecart.bin")
	if err != nil {
		t.Fatalf("Couldn't create new file to store final bin: %s", err)
	}
	arguments := []string{testPath(), "Arduboy,ArduboyFX", outbinpath, "ignore,slendemake_fx,tiles"}
	errout, err := RunLuaFlashcartGenerator(string(script), arguments, "")
	if err != nil {
		t.Fatalf("Couldn't run flashcart generator: %s. Log: \n%s", err, errout)
	}
	// Compare the two files
	testbin, err := os.ReadFile(outbinpath)
	if err != nil {
		t.Fatalf("Couldn't read %s: %s", outbinpath, err)
	}
	if !bytes.Equal(comparebin, testbin) {
		t.Fatalf("Written flashcart not equivalent! %d bytes vs %d", len(testbin), len(comparebin))
	}
}

func TestFindSuitablePackageImage(t *testing.T) {
	expected := make(map[string]string)
	expected["MicroCity.arduboy"] = "screen1.png"
	expected["TexasHoldEmFX.arduboy"] = "TexasHoldEmFX.png"
	for fname, exp := range expected {
		testfile := fileTestPath(filepath.Join(CartBuilderFolder, fname))
		archive, err := zip.OpenReader(testfile)
		if err != nil {
			t.Fatalf("Couldn't open test archive '%s': %s", testfile, err)
		}
		defer archive.Close()
		image, err := FindSuitablePackageImage(archive)
		if err != nil {
			t.Fatalf("Error finding image file: %s", err)
		}
		if image != exp {
			t.Fatalf("Unexpected image: %s vs %s", image, exp)
		}
	}
}
