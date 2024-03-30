#!/bin/bash
# WARNING: THIS TEST WILL OVERWRITE THE FLASHCART ON THE CONNECTED
# ARDUBOY DEVICE! USE AT YOUR OWN RISK

set -e

idr="ignore"
mkdir -p $idr

# Warn the user about the dangers
warningfile="$idr/youvebeenwarned"
echo "!! WARNING: THIS TEST WILL IMMEDIATELY OVERWRITE THE FLASHCART"
echo "!! ON THE CONNECTED DEVICE! USE AT YOUR OWN RISK!"

# Exit if there's no warning file
if ! [ -e "$warningfile" ]; then
	echo "---------------------------------------"
	echo "This program will now exit. Run again to actually perform the test"
	touch $warningfile
	exit 7
fi

cwd=$(pwd)
tb="testbin"
tbc="./$tb"

# Build the thing
cd ..
go build -o $tb
cd $cwd
mv ../$tb .

# Start running some tests. You MUST have an arduboy connected!
$tbc device scan | jq -e 'type=="array" and length==1'

# Test if writing + reading from 0 works
dd if=/dev/urandom of=$idr/test1.bin bs=1 count=1031
$tbc flashcart writeat any 0 -i $idr/test1.bin
$tbc flashcart readat any 0 1031 -o $idr/test1_read.bin
diff $idr/test1.bin $idr/test1_read.bin

# Test if writing + reading from a strange location works
dd if=/dev/urandom of=$idr/test2.bin bs=1 count=10301
$tbc flashcart writeat any 65427 -i $idr/test2.bin
$tbc flashcart readat any 65427 10301 -o $idr/test2_read.bin
diff $idr/test2.bin $idr/test2_read.bin
$tbc flashcart readat any 0 1031 -o $idr/test2_read1.bin
diff $idr/test1.bin $idr/test2_read1.bin

# To not leave the arduboy in a bad state, let's
# write a good flashcart and read it back to check for transparency
minicart=../testfiles/minicart.bin
$tbc flashcart write -i $minicart
$tbc flashcart readat any 0 $(wc -c <$minicart) -o $idr/testflashcartreadat.bin
diff $minicart $idr/testflashcartreadat.bin
$tbc flashcart read -o $idr/testflashcartread.bin
diff <(head -c -256 $minicart) $idr/testflashcartread.bin

# With the other crap fixed up, let's do an fxdata dev write
dd if=/dev/urandom of=$idr/testfxdev.bin bs=1 count=111104
$tbc flashcart writedev any -i $idr/testfxdev.bin
$tbc flashcart readat any 111104 111104 --fromend -o $idr/testfxdev_read.bin
diff $idr/testfxdev.bin $idr/testfxdev_read.bin

# Now write somewhere within the same block as the dev data but which doesn't
# overlap, so we can do the cool double read test like earlier
dd if=/dev/urandom of=$idr/test3.bin bs=1 count=2000
# This address is exactly 1000 off the midway point, so it will write partially
# into the previous block and partially into the next, which houses our previous
$tbc flashcart writeat any 132072 --fromend -i $idr/test3.bin
$tbc flashcart readat any 132072 2000 --fromend -o $idr/test3_read.bin
diff $idr/test3.bin $idr/test3_read.bin
$tbc flashcart readat any 111104 111104 --fromend -o $idr/test3_readfxdev.bin
diff $idr/testfxdev.bin $idr/test3_readfxdev.bin

# TODO: add the other tests you wrote down

echo "<< ALL PASS!! >>"
