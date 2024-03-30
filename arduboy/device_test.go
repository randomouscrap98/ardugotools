package arduboy

import (
	"testing"
)

func TestFitsFlashcart(t *testing.T) {
	jedec := JedecInfo{
		Capacity: FXBlockSize,
	}
	if jedec.FitsFlashcart(FXBlockSize) {
		t.Fatal("Not supposed to fit a flashcart that takes up the whole cart!")
	}
	if !jedec.FitsFlashcart(FXBlockSize - FXPageSize) {
		t.Fatal("Supposed to fit a flashcart that has a free page!")
	}
	if !jedec.FitsFlashcart(FXPageSize) {
		t.Fatal("Supposed to fit a flashcart that is just one page!")
	}
}

func TestValidateFitsFxData(t *testing.T) {
	jedec := JedecInfo{
		Capacity: 3 * FXBlockSize,
	}
	counter := 0
	check := func(fsize int, dsize int, block bool, expect bool) {
		err := jedec.ValidateFitsFxData(fsize, dsize, block)
		success := err == nil
		if success != expect {
			t.Fatalf("ERROR[%d]: %d, %d, %t, expect: %t, err: %s",
				counter, fsize, dsize, block, expect, err)
		}
		counter++
	}
	check(jedec.Capacity, 0, false, false)
	check(jedec.Capacity, 0, true, false)
	check(jedec.Capacity-FXPageSize, 0, false, true)
	check(jedec.Capacity-FXPageSize, 0, true, false)
	// Some more normal stuff
	check(FXBlockSize, FXBlockSize, false, true)
	check(FXBlockSize, FXBlockSize, true, true)
	check(FXBlockSize+1024, FXBlockSize, false, true)
	check(FXBlockSize+1024, FXBlockSize, true, true)
	// The true overlap
	check(FXBlockSize*2, FXBlockSize, false, false)
	check(FXBlockSize*2, FXBlockSize, true, false)
	// THe barely overlap
	check(FXBlockSize*2-FXPageSize, FXBlockSize, false, true)
	check(FXBlockSize*2-FXPageSize, FXBlockSize, true, false)
}
