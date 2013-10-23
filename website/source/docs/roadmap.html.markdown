---
layout: "docs"
page_title: "Product Roadmap"
sidebar_current: "docs-roadmap"
---

# Serf Project Roadmap

Serf is a young project with big ambitions. What we've built and shipped
already is a solid, powerful piece of software that solves many real world
problems. But we have
many plans to improve and iterate on Serf to make it even better. This
page outlines some of the plans we have for future versions of Serf.

Because Serf is an open source project, you as a member of the Serf
community have a big say in what features and improvements you want
to see in Serf.
If you have ideas for Serf, please feel free to post them to our
[issue tracker](https://github.com/hashicorp/serf/issues) so that we can
discuss them.

Finally, note that this roadmap is not exhaustive. We may be working on
features or changes that aren't listed here.

## Roadmap

* **Gossip message security**. In the very near term, we'll be adding
  encryption and signing to gossip-based messages, so that a bad actor
  that may know the address of your Serf cluster can't artifically inject
  messages into the cluster. Note that in most cases, this isn't an issue
  since you can protect your cluster with basic firewalls.

* **More fine-grained configuration**. The current release of Serf doesn't
  give you fine-grained control over many of the tunables of the gossip
  layer. A future version of Serf will allow you to modify these tunables
  so that Serf may work more efficiently in any environment you put it in.

* **Configuration files and directories**. The agent is currently configured
  completely from the command-line. A future version of Serf will allow you
  to specify configuration files as well as directories of files that can
  contain configuration, making Serf more friendly for configuration management
  systems.

* **SIGHUP reloading**. Alongside configuration files, Serf will respond to
  SIGHUP requests by reloading the configuration without disrupting cluster
  state.

* **Event handler library**. We think that there are many cases
  for generic event handlers. We plan on building into Serf a method of
  sharing and quickly "installing" event handlers so that you can more
  easily get Serf working with common software projects. Imagine, for example:
  `serf plugin install haproxy` to instantly get event handlers for adding
  and removing nodes to HAProxy.
