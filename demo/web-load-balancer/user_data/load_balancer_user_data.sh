#!/bin/bash
NODE_SETUP_URL="https://raw.github.com/hashicorp/serf/master/demo/web-load-balancer/setup_load_balancer.sh?login=mitchellh&token=2d79359ab0698f05fbaf1e213a7b0f92"

SERF_SETUP_URL="https://raw.github.com/hashicorp/serf/master/demo/web-load-balancer/setup_serf.sh?login=armon&token=3dafd8a678e5f1d5a621bdf64274ad01"

# Setup the node itself
wget -O - $NODE_SETUP_URL | bash

# Setup the serf agent
export SERF_ROLE="lb"
wget -O - $SERF_SETUP_URL | bash
