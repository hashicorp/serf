#!/bin/bash
NODE_SETUP_URL="https://raw.github.com/hashicorp/serf/master/demo/web-load-balancer/setup_web_server.sh?login=mitchellh&token=f489c8d8bdbd7dcc10d2dcb19c04ab0d"

SERF_SETUP_URL="https://raw.github.com/hashicorp/serf/master/demo/web-load-balancer/setup_serf.sh?login=armon&token=3dafd8a678e5f1d5a621bdf64274ad01"

# Setup the node itself
wget -O - $NODE_SETUP_URL | bash

# Setup the serf agent
export SERF_ROLE="web"
wget -O - $SERF_SETUP_URL | bash
