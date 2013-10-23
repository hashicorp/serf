---
layout: "docs"
page_title: "Agent"
sidebar_current: "docs-agent-running"
---

# Serf Agent

The Serf agent is the core process of Serf. The agent maintains membership
information, propagates events, invokes event handlers, detects failures,
and more. The agent must run on every node that is part of a Serf cluster.

## Running an Agent

The agent is started with the `serf agent` command. This command blocks,
running forever or until told to quit. The agent command takes a variety
of configuration options but the defaults are usually good enough. When
running `serf agent`, you should see output similar to that below:

```
$ serf agent
==> Starting Serf agent...
==> Serf agent running!
    Node name: 'mitchellh.local'
    Bind addr: '0.0.0.0:7946'
     RPC addr: '127.0.0.1:7373'

==> Log data will now stream in as it occurs:

2013/10/22 10:35:33 [INFO] Serf agent starting
2013/10/22 10:35:33 [INFO] serf: EventMemberJoin: mitchellh.local 127.0.0.1
2013/10/22 10:35:33 [INFO] Serf agent started
2013/10/22 10:35:33 [INFO] agent: Received event: member-join
...
```

There are three important components that `serf agent` outputs:

* **Node name**: This is a unique name for the node. By default this
  is the hostname of the machine, but you may customize it to whatever
  you'd like using the `-node` flag.

* **Bind addr**: This is the address and port used for communication between
  Serf agents in a cluster. Every Serf agent in a cluster must have the
  _same port_! If you're running multiple clusters, you'll want to choose
  a unique port per cluster.

* **RPC addr**: This is the address and port used for RPC communications
  for other `serf` commands. Other Serf commands such as `serf members`
  connect to a running agent and use RPC to query and control the agent.
  By default, this binds only to localhost on the default port. If you
  change this address, you'll have to specify an `-rpc-addr` to commands
  such as `serf members` so they know how to talk to the agent.

## Stopping an Agent

An agent can be stoped in two ways: gracefully or forcefully. To gracefully
halt an agent, send the process an interrupt signal, which is usually
`Ctrl-C` from a terminal. When gracefully exiting, the agent first notifies
the cluster it intends to leave the cluster. This way, other cluster members
notify the cluster that the node has _left_.

Alternatively, you can force kill the agent by sending it a kill signal.
When force killed, the agent ends immediately. The rest of the cluster will
eventually (usually within seconds) detect that the node has died and will
notify the cluster that the node has _failed_.

The difference between a node _failing_ and a node _leaving_ may not be
important for your use case. For example, for a web server and load
balancer setup, both result in the same action: remove the web node
from the load balancer pool. But for other situations, you may handle
each scenario differently.
