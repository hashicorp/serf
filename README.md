# Serf [![Build Status](https://github.com/hashicorp/serf/workflows/Checks/badge.svg)](https://github.com/hashicorp/serf/actions) [![Join the chat at https://gitter.im/hashicorp-serf/Lobby](https://badges.gitter.im/hashicorp-serf/Lobby.svg)](https://gitter.im/hashicorp-serf/Lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)


> [!NOTE]
> Looking for serf.io? The Serf website was shut down on 10/02/2024. The docs
previously served from serf.io can be found
[https://github.com/hashicorp/serf/blob/master/docs/index.html.markdown](https://github.com/hashicorp/serf/blob/master/docs/index.html.markdown)


* Website: https://github.com/hashicorp/serf
* Chat: [Gitter](https://gitter.im/hashicorp-serf/Lobby)
* Mailing list: [Google Groups](https://groups.google.com/group/serfdom/)

Serf is a decentralized solution for service discovery and orchestration
that is lightweight, highly available, and fault tolerant.

Serf runs on Linux, Mac OS X, and Windows. An efficient and lightweight gossip
protocol is used to communicate with other nodes. Serf can detect node failures
and notify the rest of the cluster. An event system is built on top of
Serf, letting you use Serf's gossip protocol to propagate events such
as deploys, configuration changes, etc. Serf is completely masterless
with no single point of failure.

Here are some example use cases of Serf, though there are many others:

* Discovering web servers and automatically adding them to a load balancer
* Organizing many memcached or redis nodes into a cluster, perhaps with
  something like [twemproxy](https://github.com/twitter/twemproxy) or
  maybe just configuring an application with the address of all the
  nodes
* Triggering web deploys using the event system built on top of Serf
* Propagating changes to configuration to relevant nodes.
* Updating DNS records to reflect cluster changes as they occur.
* Much, much more.

## Quick Start

First, [download a pre-built Serf binary](https://releases.hashicorp.com/serf)
for your operating system, [compile Serf yourself](#developing-serf), or install
using `go get -u github.com/hashicorp/serf/cmd/serf`.

Next, let's start a couple Serf agents. Agents run until they're told to quit
and handle the communication of maintenance tasks of Serf. In a real Serf
setup, each node in your system will run one or more Serf agents (it can
run multiple agents if you're running multiple cluster types. e.g. web
servers vs. memcached servers).

Start each Serf agent in a separate terminal session so that we can see
the output of each. Start the first agent:

```
$ serf agent -node=foo -bind=127.0.0.1:5000 -rpc-addr=127.0.0.1:7373
...
```

Start the second agent in another terminal session (while the first is still
running):

```
$ serf agent -node=bar -bind=127.0.0.1:5001 -rpc-addr=127.0.0.1:7374
...
```

At this point two Serf agents are running independently but are still
unaware of each other. Let's now tell the first agent to join an existing
cluster (the second agent). When starting a Serf agent, you must join an
existing cluster by specifying at least one existing member. After this,
Serf gossips and the remainder of the cluster becomes aware of the join.
Run the following commands in a third terminal session.

```
$ serf join 127.0.0.1:5001
...
```

If you're watching your terminals, you should see both Serf agents
become aware of the join. You can prove it by running `serf members`
to see the members of the Serf cluster:

```
$ serf members
foo    127.0.0.1:5000    alive
bar    127.0.0.1:5001    alive
...
```

At this point, you can ctrl-C or force kill either Serf agent, and they'll
update their membership lists appropriately. If you ctrl-C a Serf agent,
it will gracefully leave by notifying the cluster of its intent to leave.
If you force kill an agent, it will eventually (usually within seconds)
be detected by another member of the cluster which will notify the
cluster of the node failure.

## Documentation

Full, comprehensive documentation is viewable on the Serf website:

https://github.com/hashicorp/serf/tree/master/docs

## Developing Serf

If you wish to work on Serf itself, you'll first need [Go](https://golang.org)
installed (version 1.10+ is _required_). Make sure you have Go properly
[installed](https://golang.org/doc/install),
including setting up your [GOPATH](https://golang.org/doc/code.html#GOPATH).

Next, clone this repository into `$GOPATH/src/github.com/hashicorp/serf` and
then just type `make`. In a few moments, you'll have a working `serf` executable:

```
$ make
...
$ bin/serf
...
```

*NOTE: `make` will also place a copy of the executable under `$GOPATH/bin/`*

Serf is first and foremost a library with a command-line interface, `serf`. The
Serf library is independent of the command line agent, `serf`.  The `serf`
binary is located under `cmd/serf` and can be installed stand alone by issuing
the command `go get -u github.com/hashicorp/serf/cmd/serf`.  Applications using
the Serf library should only need to include `github.com/hashicorp/serf`.

Tests can be run by typing `make test`.

If you make any changes to the code, run `make format` in order to automatically
format the code according to Go [standards](https://golang.org/doc/effective_go.html#formatting).


 ## Metrics Emission and Compatibility

 This library can emit metrics using either `github.com/armon/go-metrics` or `github.com/hashicorp/go-metrics`. Choosing between the libraries is controlled via build tags. 

 **Build Tags**
 * `armonmetrics` - Using this tag will cause metrics to be routed to `armon/go-metrics`
 * `hashicorpmetrics` - Using this tag will cause all metrics to be routed to `hashicorp/go-metrics`

 If no build tag is specified, the default behavior is to use `armon/go-metrics`. 

 **Deprecating `armon/go-metrics`**

 Emitting metrics to `armon/go-metrics` is officially deprecated. Usage of `armon/go-metrics` will remain the default until mid-2025 with opt-in support continuing to the end of 2025.

 **Migration**
 To migrate an application currently using the older `armon/go-metrics` to instead use `hashicorp/go-metrics` the following should be done.

 1. Upgrade libraries using `armon/go-metrics` to consume `hashicorp/go-metrics/compat` instead. This should involve only changing import statements. All repositories in the `hashicorp` namespace
 2. Update an applications library dependencies to those that have the compatibility layer configured.
 3. Update the application to use `hashicorp/go-metrics` for configuring metrics export instead of `armon/go-metrics`
    * Replace all application imports of `github.com/armon/go-metrics` with `github.com/hashicorp/go-metrics`
    * Instrument your build system to build with the `hashicorpmetrics` tag.

 Eventually once the default behavior changes to use `hashicorp/go-metrics` by default (mid-2025), you can drop the `hashicorpmetrics` build tag.
