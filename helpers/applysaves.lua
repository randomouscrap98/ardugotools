-- This helper script will apply the FX saves from one cart into another cart,
-- writing it out to a third cart. This is useful for updates: you can download
-- the latest flashcart, then apply your old fx saves onto it, losing nothing
-- in the process. This is less powerful than the Arduboy Toolset's updater,
-- which allows you to keep games from both carts. This ONLY applies the saves

-- This script requires the following parameters:
-- * The path to the old flashcart (with the saves you want to keep)
-- * The path to the new flashcart
-- * The path where you want to write the final flashcart. It cannot be
--   either the new or old flashcart path, it muts be unique

local oldpath, newpath, outpath = arguments()

local oldcart = parse_flashcart(oldpath, false)
local newcart = parse_flashcart(newpath, true)
local outcart = new_flashcart(outpath)

for _, slot in ipairs(newcart) do
	-- Scan for an fxsave in the old cart if this slot at least has a title
	-- and an fxsave to replace
	if slot.fxsave and slot.title ~= "" then
		print("Found fx save for " .. slot.title .. ", scanning for old save")
		local found = false
		for _, oldslot in ipairs(oldcart) do
			if oldslot.title == slot.title and oldslot.developer == slot.developer then
				oldslot.pull_data()
				if not oldslot.fxsave or #oldslot.fxsave ~= #slot.fxsave then
					-- Not sure if people want this to be an error or not but it seems
					-- error-worthy. Usually a save doesn't randomly appear or change sizes
					error("ERROR: found matching slot but save size doesn't match!")
				else
					slot.fxsave = oldslot.fxsave
					found = true
				end
			end
		end
		if not found then
			print("WARN: couldn't find save for " .. slot.title)
		end
	end
	-- At the end of the day, just write out the slots from the new cart
	outcart.write_slot(slot)
end
