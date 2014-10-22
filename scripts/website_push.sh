#!/bin/bash

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that directory
cd $DIR

# Ensure the buildpack is setup
heroku config:set BINTRAY_API_KEY=$BINTRAY_API_KEY
heroku config:add BUILDPACK_URL=https://github.com/ddollar/heroku-buildpack-multi.git

# Push the subtree (force)
git subtree push --prefix website heroku master
