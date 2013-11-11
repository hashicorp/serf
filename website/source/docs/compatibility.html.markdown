---
layout: "docs"
page_title: "Serf Protocol Compatibility Promise"
sidebar_current: "docs-upgrading-compatibility"
---

# Protocol Compatibility Promise

We expect Serf to run in large clusters as long-running agents. Because
upgrading agents in this sort of environment relies heavily on protocol
compatibility, this page makes it clear on our promise to keeping different
Serf versions compatible with each other.

We promise that every subsequent release of Serf will remain backwards
compatible with _at least_ one prior version. Concretely: version 0.3 can
speak to 0.2 (and vice versa), but may not be able to speak to 0.1.

The backwards compatibility must be explicitly enabled: Serf agents by
default will speak the latest protocol, but can be configured to speak earlier
ones. If speaking an earlier protocol, _new features may not be available_.
The ability for an agent to speak an earlier protocol is only so that they
can be upgraded without cluster disruption.

This compatibility guarantee makes it possible to upgrade Serf agents one
at a time, one version at a time. For more details on the specifics of
upgrading, see the [upgrading page](/docs/upgrading.html).

## Protocol Compatibility Table

<table>
<tr>
<th>Version</th>
<th>Protocol Compatibility</th>
</tr>
<tr>
<td>0.1.X</td>
<td>0</td>
</tr>
<tr>
<td>0.2.X</td>
<td>0, 1</td>
</tr>
</table>
