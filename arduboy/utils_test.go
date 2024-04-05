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
