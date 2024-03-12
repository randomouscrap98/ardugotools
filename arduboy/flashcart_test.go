package arduboy

import (
	"os"
	"path/filepath"
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
