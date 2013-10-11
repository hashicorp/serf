#!/bin/sh
#
# This script installs and configures the Serf agent that runs on
# every node. As with the other scripts, this should probably be done with
# formal configuration management, but a shell script is simple as well.
#
# The SERF_ROLE environmental variable must be passed into this script
# in order to set the role of the machine. This should be either "lb" or
# "web".
#
set -e

# Download and install Serf
cd /tmp
wget -O serf.zip http://hc-ops.s3.amazonaws.com/serf/amd64.zip
unzip serf.zip
sudo mv serf /usr/local/bin/serf

# Configure the agent
cat <<EOF >/tmp/agent.conf
desc "Serf agent"

exec /usr/local/bin/serf agent \
    -event-script "member-join=/usr/local/bin/serf_member_join.sh" \
    -event-script "member-leave,member-failed=/usr/local/bin/serf_member_left.sh" \
    -role=${SERF_ROLE} >>/var/log/serf.log 2>&1
EOF
sudo mv /tmp/agent.conf /etc/init/serf.conf

# Start the agent!
sudo start serf
