-- This lua script reads the contents of a directory and creates a cart based on
-- what's found in there. The structure is simple:
-- * Each folder inside is considered a category. They are loaded in "filesystem"
--   order, which is whatever the system returns (this is usually "natural sorting")
-- * Within each folder, each .arduboy file is loaded into that category. Other files
--   are ignored
-- * Category images are set from a "title.png" file within the category folder
-- * If another folder is found within a category folder, certain files are looked
--   for to create a game named after the folder:
--   * sketch.hex  - the program (required)
--   * title.png   - image for slot (required)
--   * fxdata.bin  - fx data (optional)
--   * fxsave.bin  - fx save (optional)
--   * info.txt    - simple info with three lines: version, author, description (optional)

-- This script expects just three parameters:
-- * The path to the folder to load
-- * The device list to load from arduboy files
-- * The path to the output flashcart

local readfolder, devices, outpath = arguments()

if readfolder == nil or devices == nil or outpath == nil then
	error(
		"Must provide three parameters:\n"
			.. "* Path to the folder to load the cart from\n"
			.. "* Devices list to choose binaries from arduboy files\n"
			.. "* Path to the output flashcart\n"
			.. "Example: 'myflashcart' 'Arduboy,ArduboyFX' 'flashcart.bin'"
	)
end
