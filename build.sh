#!/bin/bash
set -e

cwd=$(pwd)
cd arduboy
go test -v
cd ../cmd/ardugotools

build() {
	builddir="$cwd/build/${1}_${2}"
	mkdir -p $builddir
	exename=ardugotools
	if [ "$1" = "windows" ]; then
		exename=ardugotools.exe
	fi
	GOOS=$1 GOARCH=$2 GO386=softfloat go build -o $builddir/$exename
	echo "Compiled $builddir"

	distdir="$cwd/dist"
	mkdir -p $distdir
	zip $distdir/ardugotools_${1}_${2}.zip $builddir/$exename
}

build windows amd64
build windows 386
build linux amd64
build linux 386
# build darwin amd64
# build darwin arm64

echo "Done"
