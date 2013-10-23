---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-agent-config"
---

# Configuration

The agent has various configuration options that can be specified via
the command-line. All of the configurations are completely optional,
and their defaults will be specified with their description below:

* `-bind` - The address that Serf will bind to for communication with
  other Serf nodes. By default this is "0.0.0.0:7946". All Serf nodes
  within a cluster must have the same port for this configuration. Serf
  uses both TCP and UDP and will use the same port for this, so if you
  have any firewalls be sure to allow both protocols. If this
  configuration value is changed and no port is specified, the default of
  "7946" will be used.

* `-event-handler` - Adds an event handler that Serf will invoke for
  events. This flag can be specified multiple times to define multiple
  event handlers. By default no event handlers are registered. See the
  [event handler page](/docs/agent/event-handlers.html) for more details on
  event handlers as well as a syntax for filtering event handlers by event.

* `-log-level` - The level of logging to show after the Serf agent has
  started. This defaults to "info". The available log levels are "trace",
  "debug", "info", "warn", "err". This is the log level that will be shown
  for the agent output, but note you can always connect via `serf monitor`
  to an agent at any log level.

* `-node` - The name of this node in the cluster. This must be unique within
  the cluster. By default this is the hostname of the machine.

* `-role` - The role of this node, if any. By default this is blank or empty.
  The role can be used by events in order to differentiate members of a
  cluster that may have different functional roles. For example, if you're
  using Serf in a load balancer and web server setup, you only want to add
  web servers to the load balancers, so the role of web servers may be "web"
  and the event handlers can filter on that.

* `-rpc-addr` - The address that Serf will bind to for the agent's internal
  RPC server. By default this is "127.0.0.1:7373", allowing only loopback
  connections. The RPC address is used by other Serf commands, such as
  `serf members`, in order to query a running Serf agent.
