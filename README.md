# Ardugotools

A simple, single-binary CLI toolset for Arduboy. Runs on many systems

## Features

- Scan for connected devices
- Analyze connected devices
- Read sketch, eeprom, flashcart
- Write sketch, eeprom, flashcart
- Write raw hex (useful for arbitrary flashing)
- Scan / parse flashcart (on device or filesystem)
- Convert between sketch hex/bin and back
- Convert title images to bin and back
- Write FX dev data
- Read and write arbitrary flashcart data at any location (useful for 
- Convert spritesheet or images to code + split to individual images

(More to come)

## Building / Using 

```
cd cmd/ardugotools
go build
./ardugotools --help
```

## Installing 

Choose one of two methods:
- Download the code and build/install manually (make sure you have [Go](https://go.dev/) installed). This is the most widely supported; use this if no release is available for your system:
  ```
  git clone https://github.com/randomouscrap98/ardugotools.git
  cd ardugotools/cmd/ardugotools
  go install
  # You should now have access to ardugotools on the command line
  ```
- Use a release binary:
  - Download one of the binaries, put it wherever you want
  - Run it locally with something like `./ardugotools` or on Windows `ardugotools.exe`. This is the easiest way: you can just copy the file into your projects and run it like that
  - To run it from anywhere, put the path to the downloaded file into your `PATH` variable
