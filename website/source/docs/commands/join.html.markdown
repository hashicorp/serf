---
layout: "docs"
page_title: "Commands: Join"
sidebar_current: "docs-commands-join"
---

# Serf Join

Command: `serf join`

The `serf join` command tells a Serf agent to join an existing cluster.
A new Serf agent must join with at least one existing member of a cluster
in order to join an existing cluster. After joining that one member,
the gossip layer takes over, propagating the updated membership state across
the cluster.

If you don't join an existing cluster, then that agent is part of its own
isolated cluster. Other nodes can join it.

Agents can join other agents multiple times without issue. If a node that
is already part of a cluster joins another node, then the clusters of the
two nodes join to become a single cluster.

## Usage

Usage: `serf join [options] address ...`

You may call join with multiple addresses if you want to try to join
multiple clusters. Serf will attempt to join all clusters, and the join
command will fail only if Serf was unable to join with any.

The command-line flags are all optional. The list of available flags are:

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:7373" which is the default RPC address of a Serf agent.

