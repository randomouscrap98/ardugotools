#!/bin/bash
set -e

cwd=$(pwd)
cd arduboy
go test -v
cd ../cmd/ardugotools

build() {
	builddir="$cwd/build/${1}_${2}"
	mkdir -p $builddir
	GOOS=$1 GOARCH=$2 GO386=softfloat go build -o $builddir/ardugotools
	echo "Compiled $builddir"

	distdir="$cwd/dist"
	mkdir -p $distdir
	zip $distdir/ardugotools_${1}_${2}.zip $builddir/ardugotools
}

build windows amd64
build windows 386
build linux amd64
build linux 386
# build darwin amd64
# build darwin arm64

echo "Done"
