-- This is an example lua script for generating a flashcart. There's more you
-- can do with this; if you need help or additional resources, let me know
-- or take a look at the provided helpers in the 'helpers' folder of the repo

-- This script expects to have the data folder set to be inside the
-- 'testfiles/cart_build' folder. You can set the folder with the -d flag
-- when running the command. An example full command to this script might be:
-- ardugotools flashcart generate -d testfiles/cart_build flashcart.lua flashcart.bin "Arduboy,ArduboyFX"

-- To make scripting easier and more reusable, you are able to read arguments
-- from the command line. These are any arguments that come after the script
-- name when you call `ardugotools flashcart generate script.lua <args>'.
-- You can pass multiple arguments; the arguments() function can return multiple
-- values. Here we pass the path to write the flashcart to, and the so-called
-- "devices" list we accept from arduboy package files.
flashpath, devices = arguments()

-- The 'new_flashcart' function begins a new flashcart and returns an object
-- which tracks the flashcart. When you write slots into the flashcart, it
-- is done against this object (in this case, 'newcart')
newcart = new_flashcart(flashpath)

-- A valid flashcart MUST start with an empty category. This is essentially
-- a placeholder required by the bootloader, so we often call it "Bootloader".
-- Each item in a flashcart is a "slot", whether it's a category or program.
-- A slot is a table with fields indicating the values to write, such as the
-- title of the slot, the image, the sketch, etc. You should always supply
-- an image for every slot, since you ONLY see the image when browsing the
-- flashcart on Arduboy. Use 'title_image' to convert any png or gif image
-- to the appropriate format. Images that aren't the right aspect ratio are
-- stretched, and colors are quantized down to just black and white using
-- a default threshold of 100.
newcart.write_slot({
	title = "Bootloader",
	image = title_image("bootloader.png"),
})

-- This is the first "real" category. All slots are written in order, and
-- games which follow a category go into that category. So, the games we
-- write next, up until we write another category, will go into this
-- so-called "Games" category
newcart.write_slot({
	title = "Games",
	image = title_image("games.png"),
})

-- Now we write programs. We are using the 'packageany' function to load a
-- package and choose the FIRST matching binary. Arduboy packages can have
-- multiple binaries to support multiple devices, and you should only choose
-- binaries which are compatible with your device. If you want to be more
-- choosy about which binary you load from the package, use the more generic
-- package() function, which takes a single device rather than a device list
-- and an optional "title" parameter for exact matches. For instance, some
-- packages have multiple binaries because they're in different languages,
-- and you may want to pick a binary based on specifically the title, which
-- hopefully indicates the language
newcart.write_slot(packageany("TexasHoldEmFX.arduboy", devices))

-- The rest of the program is just more of the same. We setup categories
-- and we setup programs, always writing them as "slots" to the flashcart.
-- You can easily generate a script like this using ANOTHER script, OR you
-- can use lua's filesystem functions to scan for files and folders and
-- create a flashcart from that. It's up to you!
newcart.write_slot(packageany("MicroCity.arduboy", devices))

-- Category
newcart.write_slot({
	title = "Horror",
	image = title_image("horror.png"),
})

-- Here's a special case: the arduboy file here doesn't have an image. The
-- package() and packageany() function returns a slot, so you can modify fields
-- in the slot after the fact. Here, we load the package, then set the image
-- since this program doesn't come with one. You could check for this
-- automatically and use a default image if you wanted to, or generate one
-- on the fly.
slot = packageany("PrinceOfArabia.V1.3.arduboy", devices)
slot.image = title_image("poa.png")
newcart.write_slot(slot)

-- You don't need to do anything at the end of the script; when the script
-- exits, all the files complete their writing and are closed.
