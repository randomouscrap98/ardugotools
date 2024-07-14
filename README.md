# Ardugotools

A simple, single-binary CLI toolset for Arduboy. Runs on many systems

## Features

- Scan / analyze connected devices
- Read / write sketch, eeprom, flashcart
- Write raw hex (useful for arbitrary flashing)
- Scan / parse flashcart (on device or filesystem)
- Convert between sketch hex/bin and back
- Convert title images to bin and back
- Write / align FX dev data
- Read and write arbitrary flashcart data at any location (useful for unique flashcart formats or custom updates)
- Convert spritesheet or images to code + split to individual images
- Generate FX data, saves, and headers using powerful lua configuration
- Generate flashcarts from `.arduboy` packages or any arbitrary data using lua scripting

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

## Flashcart generation

Like FX data generation, you can generate full flashcart files using lua scripting. 
Generating flashcarts is a complicated task that generally requires writing the entire
flashcart all at once, meaning you must know precisely how you want your flashcart to 
look, such as the categories, games, images, and order of everything. This is usually
best done with a UI, since it is inherently a visual product intended for human use.
However, for those who want **maximum control** over their flashcarts and are willing
to put in the effort, I've included a system for generating flashcarts programmatically.

In general, you will be writing a script in which you will:
- Open a new flashcart file
- One by one, create slots and add them to the flashcart 
  - Each slot is a lua table and represents either a category or a program
  - Categories just need a title and an image
  - Programs can be loaded directly from `.arduboy` files, or created manually
    by loading the individual parts, such as the hex, etc.

### Examples

Currently there is one pure example file: [flashart.lua](testfiles/flashcart.lua).
It goes over the basics of generating a flashcart using the lua script. There's more
you can do with this system though, so you may want to look at the flashcart helpers
for more examples of what you can do.

### Flashcart helpers

Since generating flashcarts is complicated, I've provided some helper scripts for
common tasks, located in the `helpers` folder. To run these, simply run the 
`ardugotools flashcart generate` command with the appropriate lua script, and 
pass in the required arguments. Each script will indicate the required arguments
if they are used incorrectly, so simply running the script with no arguments is
enough to see.

#### Add or update helper

Adding or updating an individual game in a flashcart is a very common task. This
script will attempt to add a `.arduboy` package to a flashcart in the given category.
If the program already exists in that category, it is instead updated, so you don't 
have duplicates.

You must specify the package to add, the devices you'll accept from the package, the
category, the old flashcart, and the new flashcart (can't be the same). Example:

```
ardugotools flashcart generate helpers/addorupdate.lua mygame.arduboy "Arduboy,ArduboyFX", Action, flashcart.bin newflashcart.bin
```

You can repeatedly call this script to add many files to a flashcart, though note
that it's rather inefficient to do so.

#### Apply FX saves from one flashcart into another

Another very common task is updating your flashcart. Doing so in lua is quite a chore,
so instead I've provided a script which takes fxsaves from one flashcart and applies them
to another. This way, you can download the flashcart of your choosing, perhaps from the
[cart builder website](https://www.bloggingadeadhorse.com/cart/Cart.html), then apply
the saves from your existing flashcart into the new one, letting you have all the latest
games without losing your saves.

You must specify the base flashcart (that has your saves), the flashcart to apply the saves
to (will NOT be overwritten), and the new flashcart to write to (can't be either of the 
other flashcarts). Example:

```
ardugotools flashcart generate helpers/applysaves.lua myflashcart.bin newflashcart.bin outflashcart.bin
```

Then you can flash `outflashcart.bin` to your Arduboy.
