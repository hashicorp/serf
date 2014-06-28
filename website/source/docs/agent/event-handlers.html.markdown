---
layout: "docs"
page_title: "Event Handlers"
sidebar_current: "docs-agent-events"
---

# Event Handlers

Serf's true power and flexibility comes in the form of event handlers:
scripts that are executed in response to various events that can occur
related to the Serf cluster. Serf invokes events related to membership
changes (when a node comes online or goes offline) as well as
[custom events](/docs/commands/event.html) or [queries](/docs/commands/query.html).

Event handlers can be any executable, including piped executables (such
as `awk '{print $2}' | grep foo`, since event handlers are invoked within
the context of a shell. The event handler is executed anytime an event
occurs and are expected to exit within a reasonable amount of time.

## Inputs and Parameters

Every time an event handler is invoked, Serf sets some environmental
variables:

* `SERF_EVENT` is the event type that is occuring. This will be one of
  `member-join`, `member-leave`, `member-failed`, `member-update`,
  `member-reap`, `user`, or `query`.

* `SERF_SELF_NAME` is the name of the node that is executing the event handler.

* `SERF_SELF_ROLE` is the role of the node that is executing the event handler.

* `SERF_TAG_${TAG}` is set for each tag the agent has. The tag name is upper-cased.

* `SERF_USER_EVENT` is the name of the user event if `SERF_EVENT` is "user".

* `SERF_USER_LTIME` is the `LamportTime` of the user event if `SERF_EVENT`
  is "user".

* `SERF_QUERY_NAME` is the name of the query if `SERF_EVENT` is "query".

* `SERF_QUERY_LTIME` is the `LamportTime` of the query if `SERF_EVENT`
  is "query".

In addition to these environmental variables, the data for an event is passed
in via stdin. The format of the data is dependent on the event type.

If the `SERF_EVENT` is "query", then the handler is also used to generate
a query response. If the handler exits with a 0 status code, then any output
on stdout and stderr is used as the response. The response should be of a limited
size or the response will fail to send due to size restrictions.

#### Membership Event Data

For membership related events (`member-join`, `member-leave`, `member-failed`, `member-update`, and `member-reap`),
stdin is the list of members that participated in that event. Each member is
separated by a newline and each field about the member is separated by a single
tab (`\t`). The fields of a membership event are name, address, role, then tags.
For example:

```
mitchellh.local    127.0.0.1    web    role=web,datacenter=east
```

#### User Event Data

For user events, stdin is the payload (if any) of the user event.

#### Query Data

For queries, stdin is the payload (if any) of the query.

## Specifying Event Handlers

Event handlers are specified using the `-event-handler` flag for
`serf agent`. This flag can be specified multiple times for multiple
event handlers, in which case each event handler will be executed.

Event handlers can also be filtered by event type. By default, the event
handler will be invoked for any event which may occur. But you can restrict
the events the event handler is invoked for by using a simple syntax
of `type=script`. Below are all the available ways this syntax can be
used to filter an event handler:

* `foo.sh` - The script "foo.sh" will be invoked for any/every event.

* `member-join=foo.sh` - The script "foo.sh" will only be invoked for the
  "member-join" event.

* `member-join,member-leave=foo.sh` - The script "foo.sh" will be invoked
  for either member-join or member-leave events. Any combination of events
  may be specified in this way.

* `user=foo.sh` - The script "foo.sh" will be invoked for all user events.

* `user:deploy=foo.sh` - The script "foo.sh" will be invoked only for
  "deploy" user events.

* `query=foo.sh` - The script "foo.sh" will be invoked for all queries.

* `query:load=uptime` - The uptime command will be invoked only for "load"
  queries.

