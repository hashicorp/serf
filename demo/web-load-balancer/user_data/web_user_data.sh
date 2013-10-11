#!/bin/bash
NODE_SETUP_URL="https://raw.github.com/hashicorp/serf/master/demo/web-load-balancer/setup_web_server.sh?login=mitchellh&token=f489c8d8bdbd7dcc10d2dcb19c04ab0d"

SERF_SETUP_URL="https://raw.github.com/hashicorp/serf/master/demo/web-load-balancer/setup_serf.sh?login=mitchellh&token=0f5c264420dc0cc74fc9a9b421cbddb1"

# Setup the node itself
wget -O - $NODE_SETUP_URL | bash

# Setup the serf agent
export SERF_ROLE="web"
wget -O - $SERF_SETUP_URL | bash
