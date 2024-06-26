#!/bin/sh

# This is a simple script which generates a 128x64 image usable as a title screen
# for arduboy games. You pass the text and the output file. the font used is
# the local m3x6.ttf, or you can pass an optional third parameter

# This requires imagick. It uses the new imagick command format

if [ $# -lt "2" ]; then
	echo "You must pass in the text and the output file! Example:"
	echo ""
	echo "./gentitle.sh \"This is my text\nA second line\" mytitle.png"
	echo ""
	exit 1
fi

TITLEFONT="m3x6.ttf"

if [ $# -ge "3" ]; then
	TITLEFONT="$3"
fi

magick -size 128x64 -background black -fill white -font "$TITLEFONT" -pointsize 16 -gravity center label:"$1" "$2"
