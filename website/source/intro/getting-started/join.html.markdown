---
layout: "intro"
page_title: "Join a Cluster"
sidebar_current: "gettingstarted-join"
---

# Join a Cluster

In the previous page, we started our first agent. While it showed how easy
it is to run Serf, it wasn't very excited since we simply made a cluster of
one member. In this page, we'll create a real cluster with multiple members.

When starting a Serf agent, it begins by being isolated in its own cluster.
To learn about other cluster members, the agent must _join_ an existing cluster.
To join an existing cluster, Serf only needs to know about a _single_ existing
member. After it joins, the agent will gossip with this member and quickly
determine the other members in the cluster.
