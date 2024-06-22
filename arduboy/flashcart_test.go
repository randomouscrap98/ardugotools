package arduboy

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func readTestfile(filename string) []byte {
	data, err := os.ReadFile(filepath.Join("..", "testfiles", filename))
	if err != nil {
		panic(err)
	}
	return data
}

func stdHeaderParse(t *testing.T, data []byte) *FxHeader {
	header, rest, err := ParseHeader(data)
	if err != nil {
		t.Fatalf("Error while parsing category: %s", err)
	}
	if len(data)-len(rest) != FxHeaderLength {
		t.Fatalf("Header not removed from return byteslice")
	}
	return header
}

func TestParseHeader_Category(t *testing.T) {
	data := readTestfile("header_category.bin")
	header := stdHeaderParse(t, data)
	if !header.IsCategory() {
		t.Fatalf("Expected category, was not")
	}
	if header.Category != 1 {
		t.Fatalf("Expected category to be 1, was %d", header.Category)
	}
	if header.PreviousPage != 0 {
		t.Fatalf("Expected previouspage to be 0, was %d", header.PreviousPage)
	}
	if header.NextPage != 0xA {
		t.Fatalf("Expected nextpage to be %d, was %d", 0xA, header.PreviousPage)
	}
	if header.SlotPages != 5 {
		t.Fatalf("Expected slotpages to be %d, was %d", 5, header.SlotPages)
	}
	if header.ProgramPages != 0 {
		t.Fatalf("Expected programages to be 0, was %d", header.ProgramPages)
	}
	if header.ProgramStart != 0xFFFF {
		t.Fatalf("Expected programStart to be unset, was %d", header.ProgramStart)
	}
	if header.DataStart != 0xFFFF {
		t.Fatalf("Expected datastart to be unset, was %d", header.DataStart)
	}
	if header.SaveStart != 0xFFFF {
		t.Fatalf("Expected savestart to be unset, was %d", header.SaveStart)
	}
	if header.Sha256 != "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff" {
		t.Fatalf("Expected hash to be empty, was %s", header.Sha256)
	}
	// if header.DataPages != 0 {
	// 	t.Fatalf("Expected datapages to be 0, was %d", header.DataPages)
	// }
	if header.Title != "My Games" {
		t.Fatalf("Expected title to be My Games, was %s", header.Title)
	}
	if header.Info != "Well... it's my completed games." {
		t.Fatalf("Expected info to be Well... it's my completed games, was %s", header.Info)
	}
	if header.Version != "" {
		t.Fatalf("Expected version to be empty, was %s", header.Version)
	}
	if header.Developer != "" {
		t.Fatalf("Expected developer to be empty, was %s", header.Developer)
	}
}

func TestParseHeader_Game(t *testing.T) {
	data := readTestfile("header_game.bin")
	header := stdHeaderParse(t, data)
	if header.IsCategory() {
		t.Fatalf("Expected not category, was though")
	}
	if header.Category != 2 {
		t.Fatalf("Expected category to be 2, was %d", header.Category)
	}
	if header.PreviousPage != 0x0249 {
		t.Fatalf("Expected previouspage to be %d, was %d", 0x0249, header.PreviousPage)
	}
	if header.NextPage != 0x030F {
		t.Fatalf("Expected nextpage to be %d, was %d", 0x030F, header.PreviousPage)
	}
	if header.SlotPages != 0x006A {
		t.Fatalf("Expected slotpages to be %d, was %d", 0x006A, header.SlotPages)
	}
	if header.ProgramPages != 0xCA {
		t.Fatalf("Expected programages to be %d, was %d", 0xCA, header.ProgramPages)
	}
	if header.ProgramStart != 0x02AA {
		t.Fatalf("Expected programStart to be %d, was %d", 0x02AA, header.ProgramStart)
	}
	if header.DataStart != 0xFFFF {
		t.Fatalf("Expected datastart to be unset, was %d", header.DataStart)
	}
	if header.SaveStart != 0xFFFF {
		t.Fatalf("Expected savestart to be unset, was %d", header.SaveStart)
	}
	if header.Sha256 != "a9fd7cd90a817359a798de493946e4d0c297c4868d7ec3bea21cf0b5052f3848" {
		t.Fatalf("Expected hash to be (bigstring), was %s", header.Sha256)
	}
	// if header.DataPages != 0 {
	// 	t.Fatalf("Expected datapages to be 0, was %d", header.DataPages)
	// }
	if header.Title != "Bangi" {
		t.Fatalf("Expected title to be My Games, was %s", header.Title)
	}
	if header.Version != "1.0" {
		t.Fatalf("Expected version to be 1.0, was %s", header.Version)
	}
	if header.Developer != "Igvina" {
		t.Fatalf("Expected developer to be Igvina, was %s", header.Developer)
	}
	if header.Info != "Bangi is a classic arcade game where you must shoot and destroy each of the balloons that appear in each level." {
		t.Fatalf("Expected info to be (bigstring), was %s", header.Info)
	}
}

func TestParseHeader_TooSmall(t *testing.T) {
	data := readTestfile("header_category.bin")
	data = data[:FxHeaderLength/2]
	_, _, err := ParseHeader(data)
	if err == nil {
		t.Fatalf("Expected ParseHeader to throw an error")
	}
	switch v := err.(type) {
	case *NotEnoughDataError:
		// This is correct
	default:
		t.Fatalf("Expected 'NotEnoughDataError', got %s", v)
	}
}

func TestParseHeader_NoHeader(t *testing.T) {
	data := readTestfile("header_category.bin")
	data[0] = 0
	_, _, err := ParseHeader(data)
	if err == nil {
		t.Fatalf("Expected ParseHeader to throw an error")
	}
	switch v := err.(type) {
	case *NotHeaderError:
		// This is correct
	default:
		t.Fatalf("Expected 'NotHeaderError', got %s", v)
	}
}

func testHeaderTransparent_Any(t *testing.T, filename string) {
	data := readTestfile(filename)
	header := stdHeaderParse(t, data)
	data2, err := header.MakeHeader()
	if err != nil {
		t.Fatalf("MakeHeader threw an error (%s)", filename)
	}
	if !reflect.DeepEqual(data, data2) {
		t.Fatalf("MakeHeader not transparent with ParseHeader (%s)", filename)
	}
}

func TestHeaderTransparent_Category(t *testing.T) {
	testHeaderTransparent_Any(t, "header_category.bin")
}

func TestHeaderTransparent_Game(t *testing.T) {
	testHeaderTransparent_Any(t, "header_game.bin")
}

func TestScanFlashcartFileMeta(t *testing.T) {
	file, err := os.Open(fileTestPath("minicart.bin"))
	if err != nil {
		t.Fatalf("Couldn't open test cart: %s", err)
	}
	result, err := ScanFlashcartFileMeta(file, true)
	if err != nil {
		t.Fatalf("Error scanning flashcart meta: %s", err)
	}
	if len(result) != 3 {
		t.Fatalf("Expected %d categories, got %d", 3, len(result))
	}
	slotsPerCategory := []int{0, 6, 4}
	addr := 0
	for ci, category := range result {
		if category.Address != addr {
			t.Fatalf("Expected category address %d, got %d", addr, category.Address)
		}
		// All category lengths are well known
		expectedSlotSize := FXPageSize + FxHeaderImageLength
		if category.SlotSize != expectedSlotSize {
			t.Fatalf("Expected category slotsize %d, got %d", expectedSlotSize, category.SlotSize)
		}
		if len(category.Slots) != slotsPerCategory[ci] {
			t.Fatalf("Expected %d slots in category %d, got %d", slotsPerCategory[ci], ci, len(category.Slots))
		}
		addr += category.SlotSize
		for _, program := range category.Slots {
			if program.Address != addr {
				t.Fatalf("Expected program address %d, got %d", addr, program.Address)
			}
			addr += program.SlotSize
		}
	}
}
