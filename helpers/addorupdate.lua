-- This helper script adds a .arduboy file to a category within the given
-- flashcart. If the game already exists, it is simply replaced.

-- You must pass the following arguments:
-- * the path to the .arduboy file
-- * the comma-separated list of devices you want to read from the package
-- * the name of the category to add to
-- * the path to the old flashcart
-- * the path to the new flashcart (must be different)

-- So, an example might be:
-- "mypackage.arduboy" "Arduboy,ArduboyFX" "Adventure" "oldflash.bin" "newflash.bin"

local pkgp, devs, cat, flcp, nflcp = arguments()

if pkgp == nil or devs == nil or cat == nil or flcp == nil or nflcp == nil then
	error(
		"Must provide 5 arguments to this script:\n"
			.. "* Path to .arduboy file to add\n"
			.. "* Comma-separated list of devices to accept from .arduboy file\n"
			.. "* Name of category to add arduboy file to \n"
			.. "* Path to input flashcart to add arduboy file to\n"
			.. "* Path to output flashcart (must be different than input)\n"
			.. "Example: 'mygame.arduboy' 'Arduboy,ArduboyFX' 'Action' 'originalflash.bin' 'newflash.bin'"
	)
end

local oldcart = parse_flashcart(flcp, true)
local newcart = new_flashcart(nflcp)

-- WARN: this function returns the FIRST matching binary, even if it's not
-- the most optimal. It fails if it couldn't find anything matching
local newpackage = packageany(pkgp, devs)

local insert_ready = false
local inserted = false

-- Inserting the new package
local function insert_newpackage()
	newcart.write_slot(newpackage)
	inserted = true
	insert_ready = false
end

-- This writes out each slot we find in the old cart to the new one.
-- If the slot contains our package, we simply
for _, slot in ipairs(oldcart) do
	if is_category(slot) then
		if insert_ready then
			-- We were previously ready for an insert but reached the next category
			-- and couldn't find the slot for update.
			log("Inserting new game " .. newpackage.title)
			insert_newpackage()
		elseif slot.title == cat then
			-- This is the category we care about, mark that we're inside it
			insert_ready = true
		end
	end
	-- If this is SPECIFICALLY the package to update, write the NEW
	-- package as the slot. Otherwise, just write what was in the old cart
	if
		not is_category(slot)
		and insert_ready
		and slot.title ~= ""
		and slot.title == newpackage.title
		and slot.developer == newpackage.developer
	then
		log("Updating existing game " .. slot.title)
		insert_newpackage()
	else
		newcart.write_slot(slot)
	end
end

-- Oops, we never inserted the game but the last category was where it was
-- supposed to go. This is fine
if insert_ready then
	log("Inserting new game " .. newpackage.title)
	insert_newpackage()
end

-- This is NOT fine though: the package was never inserted
if not inserted then
	error("Couldn't find category " .. cat .. " (package not inserted)")
end
