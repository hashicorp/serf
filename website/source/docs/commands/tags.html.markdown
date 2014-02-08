---
layout: "docs"
page_title: "Commands: Tags"
sidebar_current: "docs-commands-tags"
---

# Serf Tags

Command: `serf tags`

The tags command modifies a member's tags while the Serf agent is running.
The changed tags will be immediately propagated to other members in the
cluster.

Tag modifications can be persisted using Serf agent's snapshot functionality.
This is important when modifying tags dynamically so that tag values are not
lost during agent restarts.

When starting the Serf agent, any tags passed in on the command line will
overwrite any tags from previous snapshots.

## Usage

Usage: `serf tags [options]`

At least one of `-set` or `-delete` must be passed. The list of available
flags are:

* `-set` - Will either create a new tag on a member, or update it if it
  already exists with a new value. Must be passed as `-set tag=value`. Can
  be passed multiple times to set multiple tags.

* `-delete` - Delete an existing tag from a member. Can be passed multiple
  times to delete multiple tags.

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:7373" which is the default RPC address of a Serf agent.

