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
tb="$cwd/testbin"
tbc="$tb"
tfs="$cwd/../../testfiles"
# efs="$cwd/../../examples"

# Build the thing
cd ../ardugotools
go build -o $tb
cd $cwd

# These initial tests have nothing to do with the device
ofolder="$idr/fxdata"
rm -rf "$ofolder"
$tbc fxdata generate "$tfs/slendemake_fx/fxdata.lua" -d "$tfs/slendemake_fx" -o "$ofolder"
if [ ! -f "$ofolder/fxdata.h" ]; then
	echo "Expected fxdata.h to exist"
	exit 1
fi
if [ ! -f "$ofolder/fxdata_dev.bin" ]; then
	echo "Expected fxdata_dev.bin to exist"
	exit 1
fi
if [ ! -f "$ofolder/release/fxdata.bin" ]; then
	echo "Expected release/fxdata.bin to exist"
	exit 1
fi
if [ -f "$ofolder/release/fxsave.bin" ]; then
	echo "Expected no release/fxsave.bin to exist"
	exit 1
fi
diff "$ofolder/fxdata_dev.bin" "$tfs/slendemake_fx/fxdata.bin"

ofolder="$idr/fxdata2"
rm -rf "$ofolder"
$tbc fxdata generate "$tfs/fxdata.lua" -d "$tfs" -o "$ofolder"
if [ ! -f "$ofolder/fxdata.h" ]; then
	echo "Expected fxdata.h to exist"
	exit 1
fi
if [ ! -f "$ofolder/fxdata_dev.bin" ]; then
	echo "Expected fxdata_dev.bin to exist"
	exit 1
fi
if [ ! -f "$ofolder/release/fxdata.bin" ]; then
	echo "Expected release/fxdata.bin to exist"
	exit 1
fi
if [ ! -f "$ofolder/release/fxsave.bin" ]; then
	echo "Expected release/fxsave.bin to exist"
	exit 1
fi

# Now that we know some files exist, let's see if combining them again will
# yield the same file as the dev file
$tbc fxdata align -d "$ofolder/release/fxdata.bin" -s "$ofolder/release/fxsave.bin" \
	-o "$ofolder/release/fxdata_combined.bin"
diff "$ofolder/fxdata_dev.bin" "$ofolder/release/fxdata_combined.bin"

# Start running some tests. You MUST have an arduboy connected!
$tbc device scan | jq -e 'type=="array" and length==1'

# Test eeprom read/write
dd if=/dev/urandom of=$idr/testeeprom.bin bs=1 count=1024
$tbc eeprom write any -i $idr/testeeprom.bin
$tbc eeprom read any -o $idr/testeeprom_read.bin
diff $idr/testeeprom.bin $idr/testeeprom_read.bin

# test eeprom Delete
dd if=/dev/zero bs=1024 count=1 | tr "\0" "\377" >$idr/testeeprom_empty.bin
$tbc eeprom delete any
$tbc eeprom read any -o $idr/testeeprom_empty_read.bin
diff $idr/testeeprom_empty.bin $idr/testeeprom_empty_read.bin

# Test sketch bin2hex + write to device
dd if=/dev/urandom of=$idr/testsketch.bin bs=1024 count=20
$tbc sketch bin2hex -i $idr/testsketch.bin -o $idr/testsketch.hex
$tbc sketch write any -i $idr/testsketch.hex
$tbc sketch read any -o $idr/testsketch_read.hex
diff $idr/testsketch.hex $idr/testsketch_read.hex
$tbc sketch hex2bin -i $idr/testsketch_read.hex -o $idr/testsketch_read.bin
diff $idr/testsketch.bin $idr/testsketch_read.bin

# Now just write a known good hex to the device for safety
$tbc sketch write any -i "$tfs/qr-generator.hex"

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
minicart="$tfs/minicart.bin"
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
