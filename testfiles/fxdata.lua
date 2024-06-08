-- This program's fxdata configuration is a lua script. This gives you far more control
-- over the generation of fxdata, and should prevent you from having to write additionial
-- scripts to coerce data into some intermediate format (like the old fxdata.txt format).

-- You have full control over header and fxdata/save generation. You indicate when fields
-- begin within the fxdata file, then write any arbitrary data you want. There are helper
-- functions for getting your organized data into raw bytes for writing.

-- This example file shows basic usage. It produces an equivalent fxdata.bin, fxdata.sav,
-- and fxdata.h as the old fx generation programs.

----------------------------------------------------------------------------------------
-- You have various helper functions to convert data into raw:
-- * hex(str)           // convert hex string to raw data
-- * base64(str)        // convert base64 string to raw data
-- * file(path)         // load file and return raw data
-- * bytes(array, type) // convert an array of numbers to bytes, treating each field as the given type
--                      // Available types: uint8, uint16, uint23, int8, int16, int32, float32, float64
--                      // Default: uint8
----------------------------------------------------------------------------------------
-- You also have various additional functions which return specialized data:
-- * address()          // Returns the current pointer into the fxdata or fxsave binary.
--                      // Useful for manually generating pointers at arbitrary locations
-- * json(raw)          // Parse json into table. raw can be a raw string or any other raw data,
--                      // such as the results from file()
-- * image(params)      // Convert image to tilied, raw format. A complex function
--  PARAMETERS:
--   + filename         // Required: path to the image (specific filename doesn't matter)
--   + width            // Default: 0, Width of tile (0 means use all available width)
--   + height           // Default: 0, Height of tile (0 means use all available height)
--   + spacing          // Default: 0, Spacing between tiles (including on edges)
--   + usemask          // Default: false, Whether to use transparency as a data mask
--   + threshold        // Default: 100, The upper bound for black pixels
--   + alphathresh      // Default: 10, The upper bound for alpha threshold
--  RETURNS:
--   + raw data         // Write this directly to the fx flash
--   + tile count       // Amount of tiles image was split into
--   + width of tiles   // Width of each tile
--   + height of tiles  // Height of each tile
----------------------------------------------------------------------------------------
-- Then there are functions for writing data out. There are two locations data is written:
-- the header file and the binary. The binary is treated as one blob with fxdata and
-- fxsave combined, since this is technically how it's treated in the flashcart. The
-- output program then splits this for you at the end. You must write all of your fxdata
-- first, then switch to save mode if you want an fxsave. Alternatively, if you have no
-- fxdata to write and only want an fxsave, your entire lua script could be "begin_save()"
-- * field(name)        // Write out the pointer for a field with given name, using current
--                      // address. Essentially "begins" a field, after which you write the
--                      // data for that field.
-- * write(raw)         // Write raw data at the current address. Raw data is technically
--                      // a string, which all basic conversion functions listed above
--                      // (hex, base64, file, bytes) return raw data you can write. You
--                      // can write at any time, as many times as you like.
-- * header(str)        // Write raw text into the header. Useful for defining namespaces
--                      // or custom constants. You could technically write your entire header
--                      // with just header(), address(), and write(); the rest of the
--                      // functions are just helpers / shorthand
-- * pad(align, force)  // Write a bunch of 0xFF so that the entire fxdata up to this point
--                      // is aligned to the given alignment. If force = true, will add the
--                      // padding even if data is already aligned.
-- * begin_save()       // Switch to save area. If this function is not called, no save data
--                      // is generated. You must write all fxdata first, then call this
--                      // function only once and write all save data. If you are using
--                      // FX::saveGameState() and FX::loadGameState(), you simply call this
--                      // function at the end of your lua script and do no more writing.
--                      // The program will generate a blank 1-block (4096 byte) save file
--                      // as required for those functions to work. If you are using manual
--                      // saving (not recommended), you can continue writing fields and
--                      // data as normal, generating pointers into the save section.
-- * image_helper(*)    // Writing image data is tedious, though you can do it yourself
--                      // using the return values from the image() function. This function
--                      // writes the field, frame count, width and height, and then writes
--                      // all image data all at once. It is designed to be given the name
--                      // of the image, then all the output from the image() function.
--                      // For example: image_helper("mysprites", image("spritesheet.png"))
----------------------------------------------------------------------------------------

--- Begin file

image_helper("sritesheet", image("spritesheet.png", 0, 0, 0, true))

field("myhex")
write(hex("000102030405060708090A0B0C0D0E0F10"))

field("mybase64")
write(base64("SGVsbG8gd29ybGQh"))

field("mystring")
write("owo uwu !@#$%^&*()-_[]{}|;:?/.><,+=`~Z188\0")

-- Everything after this call is stored in the save section. Normally you wouldn't
-- store saves like this, but it's here to give you the option.
begin_save()

field("uneven")
write(file("uneven.bin"))
