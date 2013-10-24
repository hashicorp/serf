---
layout: "docs"
page_title: "Upgrading Serf"
sidebar_current: "docs-upgrading"
---

# Upgrading and Compatibility

Serf is meant to be a long-running agent on any nodes participating in a
Serf cluster. These nodes consistently communicate with each other. As such,
protocol level compatibility and ease of upgrades is an important thing to
keep in mind when using Serf. This page documents our policy on upgrades,
protocol changes, etc.

## Protocol Compability

As of this writing, version 0.1.x of Serf is the released version. Version
0.1.x has no notion of protocol level versioning. Version 0.2.0 of Serf
_will_ introduce versioning at the protocol level. Therefore, it is
important to know that **version 0.2.0 nodes will be incompatible with
prior versions**.

However, from 0.2.0 onwards, we will maintain compatibility between
_two_ versions, after which we'll remove that compatibility. The transitionary
version will warn when it receives messages that are deprecated. To make this
concrete, here is an example:

* Version A is released.
* Version B is released with protocol changes.
  Version B is still compatible with A, understanding messages from both
  A and B. It will warn when it receives a message from A, but will otherwise
  work.
* Version C is released. Version C can only communicate using the new
  protocol changes that B introduced, and can no longer communicate with A.
  The behavior of nodes still running the version A agent is no longer defined.

## Upgrading Serf

Upgrading Serf is simple:

1. Stop the old agent.
2. Start the new agent.

Due to the protocol compatibility guarantees made above, this will just work.
The one caveat is that the node will momentarily appear to "leave" the cluster
before quickly rejoining.
