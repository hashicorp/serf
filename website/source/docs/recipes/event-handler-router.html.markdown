---
layout: "docs"
page_title: "Serf Recipe - Event Handler Router"
sidebar_current: "docs-recipes"
---

# Recipe: Handler router

Typically you must configure a handler for each type of event you expect to
encounter (more about events and handlers
[here](/docs/agent/event-handlers.html)). To change handler configuration, one
must update the Serf configuration and reload the agent with a SIGHUP or a
restart.

Thanks to the flexibility and open-endedness Serf offers for configuring and
executing handlers, it is possible to achieve similar functionality by
configuring only a single "router" handler, which never needs to be updated.
This removes the orchestration work of reloading the agents by allowing one to
simply drop new executables with predictable names into a directory.

Handler executables must be named by event type for this recipe to work
(e.g. `member-join`). User events get prefixed with `user-` (`user-deploy`,
`user-app-reload`, etc.).

What you will end up with is a directory structure that looks like this:

```
$ tree /etc/serf
/etc/serf
└── handlers
    ├── member-failed
    ├── member-join
    ├── member-leave
    ├── member-update
    ├── user-deploy
    └── user-app-reload
```

## Handler code

The following code must be configured as the only handler for this recipe to
work. It will act as a catch-all this way and be able to make decisions on what
script handler to invoke based on the Serf environment variables.

```
#!/bin/bash
SERFDIR="/etc/serf"
[ "$SERF_EVENT" == "user" ] && EVENT="user-${SERF_USER_EVENT}" ||
EVENT="$SERF_EVENT"
HANDLER="${SERFDIR}/handlers/${EVENT}"
[ -x "$HANDLER" ] || HANDLER="${SERFDIR}/handlers/default"
exec "$HANDLER"
```
