import json
import math
import os

sbytes = 2
mapfile = os.path.join("..", "resources", "map.json")
outfile = os.path.join("..", "fx", "fxdata.txt")
entrancetile = 2
pagesprite = 10
slendersprite = 11
tilesize = 16
spriteview = 13
spritemax = 30

print(f"Loading map file {mapfile}")

# Go load that json oh yeah
with open(mapfile, "r") as f:
    jmap = json.load(f)

print(f"Scanning for relevant data")

tilelayer = None
objectlayer = None
tilegid = None
spritegid = None

# Find the layers we care about
for layer in jmap["layers"]:
    if layer["type"] == "tilelayer":
        tilelayer = layer
    elif layer["type"] == "objectgroup":
        objectlayer = layer

# Find the GIDs of the sheets so we know how to align
for tileset in jmap["tilesets"]:
    if "tile" in tileset["source"]:
        tilegid = tileset["firstgid"]
    elif "sprite" in tileset["source"]:
        spritegid = tileset["firstgid"]

assert tilelayer, "No tile layer found!"
assert objectlayer, "No object layer found!"
assert tilegid is not None, "No tileset gid found!"
assert spritegid is not None, "No spriteset gid found!"


width = tilelayer["width"]
height = tilelayer["height"]

# The values we'll be writing to
tmap = [0] * width * height
smap = [0] * width * height * sbytes
pages = [] # This will become a different structure later
slenders = [] # This will become a different structure later

for i in range(20):
    pages.append([])
    slenders.append([])

def tmi(x, y):
    """ tile map index """
    return x + y * width

def smi(x, y):
    """ sprite map index """
    return sbytes * (x + y * width)


# First, we're going to fill in the tilemap, since that's the easiest.
# Apparently it stores it 1-indexed? Weird...
print(f"Reading map data")

entrance_x = 0
entrance_y = 0

for i in range(len(tilelayer["data"])):
    mx = width - 1 - (i % width)    # We have to swap the X coordinates
    my = i // width
    mi = tmi(mx, my)
    tmap[mi] = max(0, tilelayer["data"][i] - tilegid)
    if tmap[mi] == entrancetile:
        entrance_x = mx
        entrance_y = my

print(f"Entrance at {entrance_x}, {entrance_y}")

# Then, we're going to put all the objects in the sprite map. 
print(f'Reading {len(objectlayer["objects"])} sprite data')

for obj in objectlayer["objects"]:
    # Center the location. IDK why it's in the opposite corner compared to the
    # tiles... (see the - then +)
    x = width - (obj["x"] + obj["width"] // 2) / tilesize   # Flip sprites because raycaster engine
    y = (obj["y"] - min(obj["height"] // 2, 16)) / tilesize
    # y = (obj["y"]) / tilesize
    id = max(0, obj["gid"] - spritegid)

    assert id > 0, f"Sprite ID invalid: {id}"

    # Compute the map x and y
    mapx = math.floor(x)
    mapy = math.floor(y)
    mapi = smi(mapx, mapy)
    mapfraction = math.floor(16 * (x - mapx)) + (math.floor(16 * (y - mapy)) << 4)

    # If this is a page sprite, we add it to a special thingy
    if id == pagesprite:
        locationid = int(obj["name"]) # Throws an exception if bad, which is good
        pages[locationid].append(mapx)
        pages[locationid].append(mapy)
        continue
    elif id == slendersprite:
        locationid = int(obj["name"]) # Throws an exception if bad, which is good
        slenders[locationid].append(mapx)
        slenders[locationid].append(mapy)
        # slenders[locationid].append(mapfraction)
        continue

    # See if there's already something in the sprite map. If so, fail
    assert smap[mapi] == 0, f"Two sprites in the same location: {mapx},{mapy}!"

    # Now just write it! Easy!
    smap[mapi] = id
    smap[mapi + 1] = math.floor(16 * (x - mapx)) + (math.floor(16 * (y - mapy)) << 4)
    assert smap[mapi + 1] < 256, f"Invalid smap float position at {x},{y}!"


# Find any locations that have too many sprites
for xo in range(width - spriteview):
    for yo in range(height - spriteview):
        spritetotal = 0
        for x in range(spriteview):
            for y in range(spriteview):
                if smap[smi(xo + x, yo + y)]:
                    spritetotal += 1
        assert spritetotal <= spritemax, f"Too many sprites in quadrant {xo}, {yo}: {spritetotal}"



print(f"Dumping to {outfile}")


# A function for dumping a double array, where internal arrays are uneven
# sized and you want the output to be an offset into one big array. Used
# for page and slender locations per "landmark"
def dumplocbased(basename, data, numbytes):
    print(json.dumps(data))

    indexes = []
    trueraw = []
    totalraw = 0
    f.write("uint8_t " + basename + "_raw[] = {\n")
    for pi in range(len(data)):
        if not len(data[pi]):
            continue
        indexes.append(totalraw)
        totalraw += 1
        # Write the length of the section first
        f.write(f"{len(data[pi]) // numbytes}, ")
        for v in data[pi]:
            totalraw += 1
            f.write(f"{v},")
            trueraw.append(v)
        f.write("\n")
    f.write("}\n\n")

    truepos = len(trueraw) // numbytes;
    assert truepos < 256, "Too many raw locations for indexing with a single byte!"
    print(f"{basename} positions: {truepos}")

    f.write("uint8_t " + basename + "_trueraw[] = {\n")
    f.write(f"{len(trueraw) // numbytes},\n")
    for i,tr in enumerate(trueraw):
        f.write(f"{tr},")
        if i % numbytes == numbytes - 1:
            f.write(" ")
        if i % numbytes * 10 == numbytes * 10 - 1:
            f.write("\n")
    f.write("\n}\n\n")

    f.write("uint8_t " + basename + "_offsets[] = {\n")
    for pi in indexes:
        f.write(f"{pi},")
    f.write("\n}\n\n")



with open(outfile, "w") as f:

    f.write(f"//Entrance: {entrance_x},{entrance_y}\n")

    f.write("uint8_t staticmap_fx[] = {\n")
    for y in range(height):
        for x in range(width):
            m = tmap[tmi(x,y)]
            f.write(f"{m},")
        f.write("\n")
    f.write("}\n\n")

    f.write("uint8_t staticsprites_fx[] = {\n")
    for y in range(height):
        for x in range(width):
            for i in range(sbytes):
                m = smap[smi(x,y) + i]
                f.write(f"{m},")
            f.write(" ")
        f.write("\n")
    f.write("}\n\n")

    dumplocbased("pagelocs", pages, 2)
    dumplocbased("slenderlocs", slenders, 2)

    f.write('image_t rotbg = "../resources/rotbg.png"\n\n')
    f.write('image_t rotbg_day = "../resources/rotbg_day.png"\n\n')
    f.write('image_t pages = "../resources/pages_48x64.png"\n\n')
    f.write('image_t soundgraphic = "../resources/sound_32x32.png"\n\n')
    f.write('image_t gameover = "../resources/gameover_128x64.png"\n\n')


print("Done!")
