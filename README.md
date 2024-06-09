# Ardugotools

A simple, single-binary CLI toolset for Arduboy. Runs on many systems

## Features

- Scan / analyze for connected devices
- Read / write sketch, eeprom, flashcart
- Write raw hex (useful for arbitrary flashing)
- Scan / parse flashcart (on device or filesystem)
- Convert between sketch hex/bin and back
- Convert title images to bin and back
- Write / align FX dev data
- Read and write arbitrary flashcart data at any location (useful for unique flashcart formats or custom updates)
- Convert spritesheet or images to code + split to individual images
- Generate FX data, saves, and headers using powerful lua configuration

(More to come)

## Building / Using 

Once you have the code and go installed, just run `go build` in the repo and it should produce an executable:

```shell
cd cmd/ardugotools
go build
./ardugotools --help
```
### Commands

You can get a list of all commands with `ardugotools --help`. Further information about each command can be found by running help on said command, like `ardugotools flashcart write --help`. Note that help is auto-generated, so I'm sorry for any funny formatting issues...

Many commands run against a device. You must specify the port a device is on (usually something like **COM5** on Windows or **/dev/ttyACM0**), or you can substitute `any` to connect to the first device found.

Here's some examples of what you might do:
```shell
ardugotools device scan            # See all currently connected devices
ardugotools device query any       # Get deep information about the first connected device
ardugotools sketch read any        # Read the sketch that's on the first connected device
ardugotools eeprom read COM5       # Read the eeprom that's on a particular device
ardugotools flashcart scan any --images --html > flashcart.html    # Get a webpage you can browse which shows what's on the flashcart
```

Note that for most commands, you can omit the "any" and it will still default to the first connected device.

## Installing 

Choose one of two methods:
- Download the code and build/install manually (make sure you have [Go](https://go.dev/) installed). This is the most widely supported; use this if no release is available for your system:
  ```shell
  git clone https://github.com/randomouscrap98/ardugotools.git
  cd ardugotools/cmd/ardugotools
  go install
  # You should now have access to ardugotools on the command line
  ```
- Use a release binary:
  - Download one of the binaries, put it wherever you want
  - Run it locally with something like `./ardugotools` or on Windows `ardugotools.exe`. This is the easiest way: you can just copy the file into your projects and run it like that
  - To run it from anywhere, put the path to the downloaded file into your `PATH` variable

## FX data generation

In order to utilize the FX external flash chip in the Arduboy, you would generally write
an `fxdata.txt` configuration file, which describes the layout of the data and save. You
would use the [python scripts](https://github.com/MrBlinky/Arduboy-Python-Utilities) to 
generate a header, data binary, and optionally save binary. However, I found that the format
of `fxdata.txt` was difficult to parse, and didn't give me the flexibility I desired. Most
people end up writing scripts to generate the intermediate configuration, which is then
used to generate the FX header and data.

Ardugotools attempts to shortcut this process by replacing `fxdata.txt` with `lua` scripting.
This way, you can script your data generation but then immediately write the FX
headers and data without an intermediate step. 

### Examples

There are a few example lua scripts. Please see [fxdata.lua](testfiles/fxdata.lua) for a 
thorough rundown of all the available functions as well as a small example. For a far
more complicated example, see the [slendemake script](testfiles/slendemake_fx/fxdata.lua).

