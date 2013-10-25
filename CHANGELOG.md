## 0.1.2 (Unreleased)

BUG FIXES;

  * Nodes that previously left and rejoin won't get stuck in 'leaving' state.
    [GH-18]
  * Fixing alignment issues on i386 for atomic operations [GH-20]

IMPROVEMENTS;

  * Random staggering of periodic routines to avoid cluster-wide synchronization (Memberlist)
  * Push/Pull timer automatically slows down as cluster grows to avoid congestion (Memberlist)


## 0.1.1 (October 23, 2013)

BUG FIXES;

  * Default node name is outputted when "serf agent" is called with no args.
  * Remove node from reap list after join so a fast re-join doesn't lose the
    member.

## 0.1.0 (October 23, 2013)

* Initial release
