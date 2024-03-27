#!/bin/bash
set -e

cd arduboy
go test -v
cd ..

build() {
	builddir=build/${1}_${2}
	mkdir -p $builddir
	GOOS=$1 GOARCH=$2 go build -o $builddir/ardugotools
	echo "Compiled $builddir"

	distdir=dist
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
