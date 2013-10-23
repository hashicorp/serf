---
layout: "docs"
page_title: "Commands: Event"
sidebar_current: "docs-commands-event"
---

# Serf Event

Command: `serf event`

The `serf event` command dispatches a custom user event into a Serf cluster,
leveraging Serf's gossip layer for scalable broadcasting of the event to
clusters of any size.

Nodes in the cluster can listen for these custom events and react to them.
Example use cases of custom events are to trigger deploys across web nodes
by sending a "deploy" event, possibly with a commit payload. Another use
case might be to send a "restart" event, asking the cluster members to
restart.

Ultimately, `serf event` is used to send custom events of your choosing
that you can respond to in _any way_ you want. The power in Serf's custom
events is the scalability over other systems.

## Sending an Event

To send an event, use `serf event NAME` where NAME is the name of the
event to send. This call will return immediately, and Serf will use its
gossip layer to broadcast the event.

An event may also contain a payload. You may specify the payload using
the second parameter. For example: `serf event deploy 1234567890` would
send the "deploy" event with "1234567890" as the payload.

## Receiving an Event

The events can be handled by registering an
[event handler](/docs/agent/event-handlers.html) with the Serf agent. The
documentation for how the user event is dispatched is all contained within
that linked page.

## Options

The following command-line options are available for this command.
Every option is optional:

* `-rpc-addr` - Address to the RPC server of the agent you want to contact
  to send this command. If this isn't specified, the command will contact
  "127.0.0.1:7373" which is the default RPC address of a Serf agent.

