#!/bin/bash

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that directory
cd $DIR/website

# Run the website
echo
echo "=========== INSTALLING WEBSITE DEPS ==========="
echo
bundle

echo
echo "=========== STARTING WEBSITE ==========="
echo
bundle exec middleman server
