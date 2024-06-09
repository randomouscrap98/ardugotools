-- This file emulates the "convertmap.py" script along with the subsequently
-- generated fxdata.txt file all in one. If it works correctly, you should get
-- the same binary data

local sbytes = 2
local entrancetile = 2
local pagesprite = 10
local slendersprite = 11
local tilesize = 16
local spriteview = 13
local spritemax = 30

jmap = json(file("map.json"))

local tilelayer = nil
local objectlayer = nil
local tilegid = nil
local spritegid = nil

-- Find the layers we care about
for _, layer in ipairs(jmap["layers"]) do
	if layer["type"] == "tilelayer" then
		tilelayer = layer
	elseif layer["type"] == "objectgroup" then
		objectlayer = layer
	end
end

-- Find the GIDs of the sheets so we know how to align
for _, tileset in ipairs(jmap["tilesets"]) do
	if string.find(tileset["source"], "tile") then
		tilegid = tileset["firstgid"]
	elseif string.find(tileset["source"], "sprite") then
		spritegid = tileset["firstgid"]
	end
end

assert(tilelayer ~= nil, "No tile layer found!")
assert(objectlayer ~= nil, "No object layer found!")
assert(tilegid ~= nil, "No tileset gid found!")
assert(spritegid ~= nil, "No spriteset gid found!")

local width = tilelayer["width"]
local height = tilelayer["height"]

-- The values we'll be writing to
local tmap = {}
for i = 1, width * height do
	tmap[i] = 0
end

local smap = {}
for i = 1, width * height * sbytes do
	smap[i] = 0
end

local pages = {} -- This will become a different structure later
local slenders = {} -- This will become a different structure later

for i = 1, 20 do
	table.insert(pages, {})
	table.insert(slenders, {})
end

-- tile map index
local function tmi(x, y)
	return x + y * width + 1 -- Lua uses 1-based indexing
end

-- sprite map index
local function smi(x, y)
	return sbytes * (x + y * width) + 1 -- Lua uses 1-based indexing
end

-- First, we're going to fill in the tilemap, since that's the easiest.
-- Apparently it stores it 1-indexed? Weird...
print("Reading map data")

local entrance_x = 0
local entrance_y = 0

for i = 0, #tilelayer["data"] - 1 do
	local mx = width - 1 - (i % width)
	local my = math.floor(i / width)
	local mi = tmi(mx, my)
	tmap[mi] = math.max(0, tilelayer["data"][i + 1] - tilegid) -- Adjust for 1-based index
	if tmap[mi] == entrancetile then
		entrance_x = mx
		entrance_y = my
	end
end

print(string.format("Entrance at %d, %d", entrance_x, entrance_y))

-- Then, we're going to put all the objects in the sprite map.
print(string.format("Reading %d sprite data", #objectlayer["objects"]))

for _, obj in ipairs(objectlayer["objects"]) do
	-- Center the location. IDK why it's in the opposite corner compared to the
	-- tiles... (see the - then +)
	local x = width - (obj["x"] + math.floor(obj["width"] / 2)) / tilesize -- Flip sprites because raycaster engine
	local y = (obj["y"] - math.min(math.floor(obj["height"] / 2), 16)) / tilesize
	-- y = (obj["y"]) / tilesize
	local id = math.max(0, obj["gid"] - spritegid)

	assert(id > 0, string.format("Sprite ID invalid: %d", id))

	-- Compute the map x and y
	local mapx = math.floor(x)
	local mapy = math.floor(y)
	local mapi = smi(mapx, mapy)
	-- local mapfraction = math.floor(16 * (x - mapx)) + (math.floor(16 * (y - mapy)) * 16)

	-- If this is a page sprite, we add it to a special thingy
	if id == pagesprite then
		local locationid = tonumber(obj["name"]) -- Throws an exception if bad, which is good
		table.insert(pages[locationid], mapx)
		table.insert(pages[locationid], mapy)
		goto continue
	elseif id == slendersprite then
		local locationid = tonumber(obj["name"]) -- Throws an exception if bad, which is good
		table.insert(slenders[locationid], mapx)
		table.insert(slenders[locationid], mapy)
		-- table.insert(slenders[locationid], mapfraction)
		goto continue
	end

	-- See if there's already something in the sprite map. If so, fail
	assert(smap[mapi] == 0, string.format("Two sprites in the same location: %d,%d!", mapx, mapy))

	-- Now just write it! Easy!
	smap[mapi] = id
	smap[mapi + 1] = math.floor(16 * (x - mapx)) + (math.floor(16 * (y - mapy)) * 16)
	assert(smap[mapi + 1] < 256, string.format("Invalid smap float position at %f,%f!", x, y))

	::continue::
end

-- Find any locations that have too many sprites
for xo = 0, width - spriteview - 1 do
	for yo = 0, height - spriteview - 1 do
		local spritetotal = 0
		for x = 0, spriteview - 1 do
			for y = 0, spriteview - 1 do
				if smap[smi(xo + x, yo + y)] ~= 0 then
					spritetotal = spritetotal + 1
				end
			end
		end
		assert(spritetotal <= spritemax, string.format("Too many sprites in quadrant %d, %d: %d", xo, yo, spritetotal))
	end
end

-- ------------------------------
-- Dump the data to the fx
-- ------------------------------

field("staticmap_fx")
write(bytes(tmap))
field("staticsprites_fx")
write(bytes(smap))

-- Some complicated thing...

-- And then the normal image junk
image_helper("rotbg", image("rotbg.png"))
image_helper("rotbg_day", image("rotbg_day.png"))
image_helper("pages", image("pages_48x64.png", 48, 64))
image_helper("soundgraphic", image("sound_32x32.png", 32, 32))
image_helper("gameover", image("gameover_128x64.png", 128, 64))
