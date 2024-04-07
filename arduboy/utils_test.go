package arduboy

import (
	"testing"
)

func testAlignWidth(width uint, align uint, expected uint, t *testing.T) {
	result := AlignWidth(width, align)
	if result != expected {
		t.Fatalf("%d align %d: Expected %d, got %d", width, align, expected, result)
	}
}

func TestAlignWidth_All(t *testing.T) {
	testAlignWidth(5, 1024, 1024, t)
	testAlignWidth(0, 1024, 0, t)
	testAlignWidth(1024, 1024, 1024, t)
	testAlignWidth(255, 256, 256, t)
	testAlignWidth(257, 256, 512, t)
	testAlignWidth(511, 256, 512, t)
	testAlignWidth(513, 256, 512+256, t)
	testAlignWidth(33, 4, 36, t)
	testAlignWidth(34, 4, 36, t)
	testAlignWidth(35, 4, 36, t)
	testAlignWidth(36, 4, 36, t)
	testAlignWidth(37, 4, 40, t)
}

func TestMakePadding(t *testing.T) {
	result := MakePadding(1)
	if len(result) != 1 {
		t.Fatalf("Expected exactly one byte!")
	}
	if result[0] != 0xFF {
		t.Fatalf("Expected one byte to be 0xFF!")
	}
	result = MakePadding(233)
	if len(result) != 233 {
		t.Fatalf("Expected exactly 233 bytes!")
	}
	for i := range result {
		if result[i] != 0xFF {
			t.Fatalf("Expected byte [%d] to be 0xFF, was %d!", i, result[i])
		}
	}
}

func TestSortedKeys(t *testing.T) {
	keys := []string{"cows", "NO", "chickens", "sheep"}
	ti := func(d []string, index int, key string) {
		if d[index] != key {
			t.Fatalf("Expected %s at %d, was %s", key, index, d[index])
		}
	}
	// I'm hoping that the random keys will manifest within the loop
	for i := 0; i < 100; i++ {
		dic := make(map[string]*int)
		dic["a"] = nil
		dic["b"] = nil
		dic["chickens"] = nil
		dic["sheep"] = nil
		dic["cows"] = nil
		sorted := SortedKeys(dic, keys)
		if len(sorted) != len(dic) {
			t.Fatalf("SortedKeys not right length. Expected %d, got %d", len(dic), len(sorted))
		}
		ti(sorted, 0, "cows")
		ti(sorted, 1, "chickens")
		ti(sorted, 2, "sheep")
		if (sorted[3] != "a" && sorted[3] != "b") || (sorted[4] != "a" && sorted[4] != "b") {
			t.Fatalf("SortedKeys did not contain the non-sorted keys!")
		}
	}
}
