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

sudo apt-get install -y unzip

# Download and install Serf
cd /tmp
wget -O serf.zip http://hc-ops.s3.amazonaws.com/serf/amd64.zip
unzip serf.zip
sudo mv serf /usr/local/bin/serf

# The member join script is invoked when a member joins the Serf cluster.
# Our join script simply adds the node to the load balancer.
cat <<EOF >/tmp/join.sh
if [ "x${SERF_SELF_ROLE}" != "xlb" ]; then
    echo "Not an lb. Ignoring member join."
    exit 0
fi

while read line; do
    echo $line | \
        awk '{ printf "    server %s %s\n", $1, $2 }' >/etc/haproxy/haproxy.cfg
done

/etc/init.d/haproxy reload
EOF
sudo mv /tmp/join.sh /usr/local/bin/serf_member_join.sh
chmod +x /usr/local/bin/serf_member_join.sh

# The member leave script is invoked when a member leaves or fails out
# of the serf cluster. Our script removes the node from the load balancer.
cat <<EOF >/tmp/leave.sh
if [ "x${SERF_SELF_ROLE}" != "xlb" ]; then
    echo "Not an lb. Ignoring member leave"
    exit 0
fi

while read line; do
    NAME=`echo $line | awk '{print \$1 }'`
done

/etc/init.d/haproxy reload
EOF
sudo mv /tmp/leave.sh /usr/local/bin/serf_member_left.sh
chmod +x /usr/local/bin/serf_member_left.sh

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
