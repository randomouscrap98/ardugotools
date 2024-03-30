#!/bin/bash

set -e

cwd=$(pwd)
tb="testbin"
tbc="./$tb"
idr="ignore"
mkdir -p $idr

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

# As a final test (to not leave the arduboy in a bad state), let's
# write a good flashcart and read it back to check for transparency
minicart=../testfiles/minicart.bin
$tbc flashcart write -i $minicart
$tbc flashcart readat any 0 $(wc -c <$minicart) -o $idr/testflashcartreadat.bin
diff $minicart $idr/testflashcartreadat.bin
$tbc flashcart read -o $idr/testflashcartread.bin
diff <(head -c -256 $minicart) $idr/testflashcartread.bin

echo "<< ALL PASS!! >>"
