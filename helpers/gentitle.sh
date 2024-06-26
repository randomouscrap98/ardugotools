#!/bin/sh

# This is a simple script which generates a 128x64 image usable as a title screen
# for arduboy games. You pass the text and the output file. the font used is
# the local m3x6.ttf

# This requires imagick. It uses the new imagick command format

if [ $# -ne "2" ]; then
	echo "You must pass in the text and the output file! Example:"
	echo ""
	echo "./gentitle.sh \"This is my text\nA second line\" mytitle.png"
	echo ""
	exit 1
fi

magick -size 128x64 -background black -fill white -font m3x6.ttf -pointsize 16 -gravity center label:"$1" "$2"
