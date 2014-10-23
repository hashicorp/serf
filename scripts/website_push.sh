#!/bin/bash

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that directory
cd $DIR

# If bootstrap is on, this is the first push
if [ "${BOOTSTRAP}" != "" ]; then
    heroku config:set BINTRAY_API_KEY=$BINTRAY_API_KEY
    heroku config:add BUILDPACK_URL=https://github.com/ddollar/heroku-buildpack-multi.git
fi

# Push the subtree (force)
git push heroku `git subtree split --prefix website master`:master --force
