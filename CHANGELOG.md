## 0.3.1 (Unreleased)

BUG FIXES:

 * `leave_on_interrupt` set to false when not specified, if
 any config file is provided. This flag is deprecated for
 `skip_leave_on_interrupt` instead. [GH-94]

## 0.3.0 (December 5, 2013)

FEATURES:

  * Dynamic port support, cluster wide consistent config not necessary
  * Snapshots to automaticaly rejoin cluster after failure and prevent replays [GH-84] [GH-71]
  * Adding `profile` config to agent, to support WAN, LAN, and Local modes
  * MsgPack over TCP RPC protocol which can be used to control Serf, send events, and
  receive events with low latency.
  * New `leave` CLI command and RPC endpoint to control graceful leaves
  * Signal handling is controlable, graceful leave behavior on SIGINT/SIGTERM
  can be specified
  * SIGHUP can be used to reload configuration

IMPROVEMENTS:

  * Event handler provides lamport time of user events via SERF_USER_LTIME [GH-68]
  * Memberlist encryption overhead has been reduced
  * Filter output of `members` using regular expressions on role and status
  * `replay_on_join` parameter to control replay with `start_join`
  * `monitor` works even if the client is behind a NAT
  * Serf generates warning if binding to public IP without encryption

BUG FIXES:

  * Prevent unbounded transmit queues [GH-78]
  * IPv6 addresses can be bound to [GH-72]
  * Serf join won't hang on a slow/dead node [GH-70]
  * Serf Leave won't block Shutdown [GH-1]

## 0.2.1 (November 6, 2013)

BUG FIXES:

  * Member role and address not updated on re-join [GH-58]

## 0.2.0 (November 1, 2013)

FEATURES:

  * Protocol versioning features so that upgrades can be done safely.
    See the website on upgrading Serf for more info.
  * Can now configure Serf with files or directories of files by specifying
    the `-config-file` and/or `-config-dir` flags to the agent.
  * New command `serf force-leave` can be used to force a "failed" node
    to the "left" state.
  * Serf now supports message encryption and verification so that it can
    be used on untrusted networks [GH-25]
  * The `-join` flag on `serf agent` can be used to join a cluster when
    starting an agent. [GH-42]

IMPROVEMENTS:

  * Random staggering of periodic routines to avoid cluster-wide
    synchronization
  * Push/Pull timer automatically slows down as cluster grows to avoid
    congestion
  * Messages are compressed to reduce bandwidth utilization
  * `serf members` now provides node roles in output
  * Joining a cluster will no longer replay all the old events by default,
    but it can using the `-replay` flag.
  * User events are coalesced by default, meaning duplicate events (by name)
    within a short period of time are merged. [GH-8]

BUG FIXES:

  * Event handlers work on Windows now by executing commands through
    `cmd /C` [GH-37]
  * Nodes that previously left and rejoin won't get stuck in 'leaving' state.
    [GH-18]
  * Fixing alignment issues on i386 for atomic operations [GH-20]
  * "trace" log level works [GH-31]

## 0.1.1 (October 23, 2013)

BUG FIXES:

  * Default node name is outputted when "serf agent" is called with no args.
  * Remove node from reap list after join so a fast re-join doesn't lose the
    member.

## 0.1.0 (October 23, 2013)

* Initial release
