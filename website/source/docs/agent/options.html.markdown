---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-agent-config"
---

# Configuration

The agent has various configuration options that can be specified via
the command-line or via configuration files. All of the configuration
options are completely optional and their defaults will be specified
with their descriptions.

When loading configuration, Serf loads the configuration from files
and directories in the order specified. Configuration specified later
will be merged into configuration specified earlier. In most cases,
"merge" means that the later version will override the earlier. But in
some cases, such as event handlers, merging just appends the handlers.
The exact merging behavior will be specified.

## Command-line Options

The options below are all specified on the command-line.

* `-bind` - The address that Serf will bind to for communication with
  other Serf nodes. By default this is "0.0.0.0:7946". All Serf nodes
  within a cluster must have the same port for this configuration. Serf
  uses both TCP and UDP and will use the same port for this, so if you
  have any firewalls be sure to allow both protocols. If this
  configuration value is changed and no port is specified, the default of
  "7946" will be used.

* `-config-file` - A configuration file to load. For more information on
  the format of this file, read the "Configuration Files" section below.
  This option can be specified multiple times to load multiple configuration
  files. If it is specified multiple times, configuration files loaded later
  will merge with configuration files loaded earlier, with the later values
  overriding the earlier values.

* `-config-dir` - A directory of configuration files to load. Serf will
  load all files in this directory ending in ".json" as configuration files
  in alphabetical order. For more information on the format of the configuration
  files, see the "Configuration Files" section below.

* `-encrypt` - Specifies the secret key to use for encryption of Serf
  network traffic. This key must be 16-bytes that are base64 encoded. The
  easiest way to create an encryption key is to use `serf keygen`. All
  nodes within a cluster must share the same encryption key to communicate.

* `-event-handler` - Adds an event handler that Serf will invoke for
  events. This flag can be specified multiple times to define multiple
  event handlers. By default no event handlers are registered. See the
  [event handler page](/docs/agent/event-handlers.html) for more details on
  event handlers as well as a syntax for filtering event handlers by event.

* `-join` - Address of another agent to join upon starting up. This can be
  specified multiple times to specify multiple agents to join. If Serf is
  unable to join with any of the specified addresses, agent startup will
  fail. By default, the agent won't join any nodes when it starts up.

* `-log-level` - The level of logging to show after the Serf agent has
  started. This defaults to "info". The available log levels are "trace",
  "debug", "info", "warn", "err". This is the log level that will be shown
  for the agent output, but note you can always connect via `serf monitor`
  to an agent at any log level.

* `-node` - The name of this node in the cluster. This must be unique within
  the cluster. By default this is the hostname of the machine.

* `-protocol` - The Serf protocol version to use. This defaults to the latest
  version. This should be set only when [upgrading](/docs/upgrading.html).
  You can view the protocol versions supported by Serf by running `serf -v`.

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

## Configuration Files

In addition to the command-line options, configuration can be put into
files. This may be easier in certain situations, for example when Serf is
being configured using a configuration management system.

The configuration files are JSON formatted, making them easily readable
and editable by both humans and computers. The configuration is formatted
at a single JSON object with configuration within it.

#### Example Configuration File

<pre class="prettyprint lang-json">
{
  "role": "load-balancer",

  "event_handlers": [
    "handle.sh",
    "user:deploy=deploy.sh"
  ]
}
</pre>

#### Configuration Key Reference

* `node_name` - Equivalent to the `-node` command-line flag.

* `role` - Equivalent to the `-role` command-line flag.

* `bind` - Equivalent to the `-bind` command-line flag.

* `encrypt_key` - Equivalent to the `-encrypt` command-line flag.

* `log_level` - Equivalent to the `-log-level` command-line flag.

* `protocol` - Equivalent to the `-protocol` command-line flag.

* `rpc_addr` - Equivalent to the `-rpc-addr` command-line flag.

* `event_handlers` - An array of strings specifying the event handlers.
  The format of the strings is equivalent to the format specified for
  the `-event-handler` command-line flag.

* `start_join` - An array of strings specifying addresses of nodes to
  join upon startup.
