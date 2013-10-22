---
layout: "intro"
page_title: "Run the Agent"
sidebar_current: "gettingstarted-agent"
---

# Run the Serf Agent

After Serf is installed, the agent must be run. The agent is a lightweight
process that runs forever (until told to quit) and maintains cluster membership
and communication. The agent must be run for every node that will be part of
the cluster.

In some cases, multiple agents will need to be run if you're running multiple
Serf clusters. For example, you may want to run a separate Serf cluster to
maintain web server membership info for a load balancer from another Serf
cluster that manages membership of Memcached nodes, but perhaps the web
servers need to be part of the Memcached cluster too so they can be notified
when Memcached nodes come online or go offline.

For simplicity, we'll run a single Serf agent right now:

```
$ serf agent
==> Starting Serf agent...
==> Serf agent running!
    Node name: ''
    Bind addr: '0.0.0.0:7946'
     RPC addr: '127.0.0.1:7373'

==> Log data will now stream in as it occurs:

2013/10/21 18:57:15 [INFO] Serf agent starting
2013/10/21 18:57:15 [INFO] serf: EventMemberJoin: mitchellh.local 10.0.1.60
2013/10/21 18:57:15 [INFO] Serf agent started
2013/10/21 18:57:15 [INFO] agent: Received event: member-join
```

As you can see, the Serf agent has started and has outputted some log
data. From the log data, you can see that a member has joined the cluster.
This member is yourself.

You can use `Ctrl-C` (the interrupt signal) to gracefully halt the agent.
After interrupting the agent, you should see it leave the cluster gracefully
and shut down.

By gracefully leaving, Serf would notify other cluster members that the
node _left_. If you had forcibly killed the agent process, other members
of the cluster would have detected that the node _failed_. This can be a
crucial difference depending on what your use case of Serf is.
