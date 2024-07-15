-- This lua script reads the contents of a directory and creates a cart based on
-- what's found in there. The structure is simple:
-- * Each folder inside is considered a category. They are loaded in "filesystem"
--   order, which is whatever the system returns (this is usually "natural sorting")
-- * Within each folder, each .arduboy file is loaded into that category. Other files
--   are ignored
--   * The packages MUST have an image to use as a title card. If one is not found,
--     a PNG file is checked next to the package with the same name. For instance,
--     if you have PrinceOfArabia1.3.arduboy, it will look for PrinceOfArabia1.3.png
-- * Category images are set from a "title.png" file within the category folder
-- * If another folder is found within a category folder, certain files are looked
--   for to create a game named after the folder:
--   * sketch.hex  - the program (required)
--   * title.png   - image for slot (required)
--   * fxdata.bin  - fx data (optional)
--   * fxsave.bin  - fx save (optional)
--   * info.txt    - simple info with three lines: version, author, description (optional)
-- * The bootloader category is added automatically. The image is taken from a
--   title.png in the root of the specified cart directory. This is required.

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

-- This is a provided function from the go environment: it gives you a list of
-- tables with information on each file and folder in the directory
local maindirlist = listdir(readfolder)

local flashcart = new_flashcart(outpath)

-- Return the loaded and parsed title image. Returns nil if couldn't find
local function load_title(dirlist, name)
	-- Search for the title. Return nothing if not found
	for _, dinfo in ipairs(dirlist) do
		if dinfo.name == name then
			return title_image(dinfo.path)
		end
	end
end

-- Check a slot for validity THEN add it. Throws an error if the slot is invalid
local function add_slot(slot)
	if slot.image == nil then
		error("Slot " .. slot.title .. " MUST have an image! You won't be able to tell what it is otherwise!")
	end
	flashcart.write_slot(slot)
end

-- Add a category to the running flashcart
local function add_category(name, dirlist)
	local slot = {
		title = name,
		image = load_title(dirlist, "title.png"),
	}
	add_slot(slot)
end

add_category("Bootloader", maindirlist)

-- Iterate over all categories. Well, they MIGHT be categories...
for _, catdir in ipairs(maindirlist) do
	if not catdir.is_directory then
		log("Unexpected file: " .. catdir.path)
		goto skipdir
	end
	-- List files within the category. These are most likely the games
	local catlist = listdir(catdir.path)
	add_category(catdir.name, catlist)
	-- Now iterate over all the stuff inside the category and load appropriate stuff
	for _, catfile in ipairs(catdir) do
		if catfile.is_directory then
			-- This is one of those weird folder-based games.
			log("Non-packaged programs not supported at this time")
			goto skipfile
		elseif string.sub(catfile.name, -8) == ".arduboy" then
			-- This is a normal arduboy package
			slot = packageany(catfile.path, devices)
			if slot.image == nil then
				-- Try to find an image with the same name as the package.
				slot.image = load_title(catdir, string.sub(catfile.name, 0, -8) .. "png")
			end
			add_slot(slot)
		elseif catfile.name ~= "title.png" then
			-- This is something unexpected!
			log("Unexpected file: " .. catdir.path)
			goto skipfile
		end
		::skipfile::
	end
	::skipdir::
end
