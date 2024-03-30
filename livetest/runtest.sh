#!/bin/bash

set -e

cwd=$(pwd)
tb="testbin"
tbc="./$tb"

# Build the thing
cd ..
go build -o $tb
cd $cwd
mv ../$tb .

# Start running some tests. You MUST have an arduboy connected!
$tbc device scan | jq -e 'type=="array" and length==1'

echo "All pass"
