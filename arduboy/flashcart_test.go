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
		t.Errorf("Error while parsing category: %s", err)
	}
	if len(data)-len(rest) != FxHeaderLength {
		t.Errorf("Header not removed from return byteslice")
	}
	return header
}

func TestParseHeader_Category(t *testing.T) {
	data := readTestfile("header_category.bin")
	header := stdHeaderParse(t, data)
	if !header.IsCategory() {
		t.Errorf("Expected category, was not")
	}
	if header.Category != 1 {
		t.Errorf("Expected category to be 1, was %d", header.Category)
	}
	if header.PreviousPage != 0 {
		t.Errorf("Expected previouspage to be 0, was %d", header.PreviousPage)
	}
	if header.NextPage != 0xA {
		t.Errorf("Expected nextpage to be %d, was %d", 0xA, header.PreviousPage)
	}
	if header.SlotPages != 5 {
		t.Errorf("Expected slotpages to be %d, was %d", 5, header.SlotPages)
	}
	if header.ProgramPages != 0 {
		t.Errorf("Expected programages to be 0, was %d", header.ProgramPages)
	}
	if header.ProgramStart != 0xFFFF {
		t.Errorf("Expected programStart to be unset, was %d", header.ProgramStart)
	}
	if header.DataStart != 0xFFFF {
		t.Errorf("Expected datastart to be unset, was %d", header.DataStart)
	}
	if header.SaveStart != 0xFFFF {
		t.Errorf("Expected savestart to be unset, was %d", header.SaveStart)
	}
	if header.Sha256 != "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff" {
		t.Errorf("Expected hash to be empty, was %s", header.Sha256)
	}
	// if header.DataPages != 0 {
	// 	t.Errorf("Expected datapages to be 0, was %d", header.DataPages)
	// }
	if header.Title != "My Games" {
		t.Errorf("Expected title to be My Games, was %s", header.Title)
	}
	if header.Info != "Well... it's my completed games." {
		t.Errorf("Expected info to be Well... it's my completed games, was %s", header.Info)
	}
	if header.Version != "" {
		t.Errorf("Expected version to be empty, was %s", header.Version)
	}
	if header.Developer != "" {
		t.Errorf("Expected developer to be empty, was %s", header.Developer)
	}
}

func TestParseHeader_Game(t *testing.T) {
	data := readTestfile("header_game.bin")
	header := stdHeaderParse(t, data)
	if header.IsCategory() {
		t.Errorf("Expected not category, was though")
	}
	if header.Category != 2 {
		t.Errorf("Expected category to be 2, was %d", header.Category)
	}
	if header.PreviousPage != 0x0249 {
		t.Errorf("Expected previouspage to be %d, was %d", 0x0249, header.PreviousPage)
	}
	if header.NextPage != 0x030F {
		t.Errorf("Expected nextpage to be %d, was %d", 0x030F, header.PreviousPage)
	}
	if header.SlotPages != 0x006A {
		t.Errorf("Expected slotpages to be %d, was %d", 0x006A, header.SlotPages)
	}
	if header.ProgramPages != 0xCA {
		t.Errorf("Expected programages to be %d, was %d", 0xCA, header.ProgramPages)
	}
	if header.ProgramStart != 0x02AA {
		t.Errorf("Expected programStart to be %d, was %d", 0x02AA, header.ProgramStart)
	}
	if header.DataStart != 0xFFFF {
		t.Errorf("Expected datastart to be unset, was %d", header.DataStart)
	}
	if header.SaveStart != 0xFFFF {
		t.Errorf("Expected savestart to be unset, was %d", header.SaveStart)
	}
	if header.Sha256 != "a9fd7cd90a817359a798de493946e4d0c297c4868d7ec3bea21cf0b5052f3848" {
		t.Errorf("Expected hash to be (bigstring), was %s", header.Sha256)
	}
	// if header.DataPages != 0 {
	// 	t.Errorf("Expected datapages to be 0, was %d", header.DataPages)
	// }
	if header.Title != "Bangi" {
		t.Errorf("Expected title to be My Games, was %s", header.Title)
	}
	if header.Version != "1.0" {
		t.Errorf("Expected version to be 1.0, was %s", header.Version)
	}
	if header.Developer != "Igvina" {
		t.Errorf("Expected developer to be Igvina, was %s", header.Developer)
	}
	if header.Info != "Bangi is a classic arcade game where you must shoot and destroy each of the balloons that appear in each level." {
		t.Errorf("Expected info to be (bigstring), was %s", header.Info)
	}
}

func TestParseHeader_TooSmall(t *testing.T) {
	data := readTestfile("header_category.bin")
	data = data[:FxHeaderLength/2]
	_, _, err := ParseHeader(data)
	if err == nil {
		t.Errorf("Expected ParseHeader to throw an error")
	}
	switch v := err.(type) {
	case *NotEnoughDataError:
		// This is correct
	default:
		t.Errorf("Expected 'NotEnoughDataError', got %s", v)
	}
}

func TestParseHeader_NoHeader(t *testing.T) {
	data := readTestfile("header_category.bin")
	data[0] = 0
	_, _, err := ParseHeader(data)
	if err == nil {
		t.Errorf("Expected ParseHeader to throw an error")
	}
	switch v := err.(type) {
	case *NotHeaderError:
		// This is correct
	default:
		t.Errorf("Expected 'NotHeaderError', got %s", v)
	}
}

func testHeaderTransparent_Any(t *testing.T, filename string) {
	data := readTestfile(filename)
	header := stdHeaderParse(t, data)
	data2, err := header.MakeHeader()
	if err != nil {
		t.Errorf("MakeHeader threw an error (%s)", filename)
	}
	if !reflect.DeepEqual(data, data2) {
		t.Errorf("MakeHeader not transparent with ParseHeader (%s)", filename)
	}
}

func TestHeaderTransparent_Category(t *testing.T) {
	testHeaderTransparent_Any(t, "header_category.bin")
}

func TestHeaderTransparent_Game(t *testing.T) {
	testHeaderTransparent_Any(t, "header_game.bin")
}
