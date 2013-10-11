#!/bin/bash
SCRIPT_URL="https://raw.github.com/hashicorp/serf/master/demo/setup_load_balancer.sh?login=mitchellh&token=6dcf0bcc793ca437da6d77746fb810b7"

wget -O - $SCRIPT_URL | bash
