# Sections

## Gossip-based Membership

Serf relies on an efficient and lightweight gossip protocol
to communicate with nodes. The Serf agents periodically exchange
messages with each other in much the same way that a zombie
apocalypse would occur: it starts with one zombie but soon
infects everyone. In practice, the gossip is [very fast
and extremely efficient](#).

## Failure Detection

Serf is able to quickly detect failed members and notify the
rest of the cluster. This failure detection is built into
the heart of the gossip protocol used by Serf. Like humans
in a zombie apocalypse, everybody checks their peers for
infection and quickly alerts the other living humans. Serf
relies on a random probing technique which is proven to
efficiently scale to clusters of any size.

## Custom Events

In addition to managing membership, Serf can broadcast custom events.
These can be used to trigger deploys, restart processes, spread tales
of human heroism, and anything else you may want. The event system is
flexible and lightweight, making it easy for application developers and
sysadmins alike to leverage.
