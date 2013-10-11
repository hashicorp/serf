#!/bin/bash
NODE_SETUP_URL="https://raw.github.com/hashicorp/serf/master/demo/setup_web_server.sh?login=mitchellh&token=894b69f833522a8d3c335c40ac99fa6d"

SERF_SETUP_URL="https://raw.github.com/hashicorp/serf/master/demo/setup_serf.sh?login=mitchellh&token=09af864f2bdfef4ebdd9245a02177991"

# Setup the node itself
wget -O - $NODE_SETUP_URL | bash

# Setup the serf agent
export SERF_ROLE="web"
wget -O - $SERF_SETUP_URL | bash
