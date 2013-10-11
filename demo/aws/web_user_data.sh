#!/bin/bash
SCRIPT_URL="https://raw.github.com/hashicorp/serf/master/demo/setup_web_server.sh?login=mitchellh&token=894b69f833522a8d3c335c40ac99fa6d"

wget -O - $SCRIPT_URL | bash
