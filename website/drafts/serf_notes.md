Serf
* distributed
* fault tolerant
* partition healing
* orchestration
* events
* service
* tool
* cloud?
* service discovery
* highly availability

Serf is a tool for making service orchestration and management
simple and fault-tolerant

Serf is a decentralized tool for managing and ochestrating services

Serf is a flexible tool for service organization and management
designed for the modern cloud

Serf is a tool built for the modern cloud to make service
ochestration and management simple

Serf is a decentralized tool for service orchestration that is
fault-tolerant, simple, and DevOps friendly
----
* Membership
* Failure detection
* Events

The Serf agent works by periodically sending and receiving messages
about the members of the gossip pool. This process is lightweight
with resource use independent of cluster size, but still converges
rapidly by relying on a probabilistic random broadcast.

--------

Serf relies on an efficient and lightweight gossip protocol to
communicate between nodes. The Serf agents periodically exchange
messages allowing information to rapidly spread through the cluster.
In practice, this works like a zombie apocalyse, it starts with one
zombie but soon all humans are infected.

--------

Failure detection is build into the heart of the gossip protocol
used by Serf. Like humans in a zombie apocolyse, everybody checks
their peers for infection and quickly alert the other living humans.
Serf relies on a random probing technique which is proven to
efficiently scale to clusters of any size.

--------

In addition to managing membership, Serf can broadcast custom events.
These can be used to trigger deploys, restart processes, spread
tales of human heroism, and anything else. The event system is flexible
and lightweight, making it easy for application developers and sysadmins
alike to leverage.

# Use Cases

* Discovering web servers for load balancers
* Organizing memcache or redis nodes into a cluster
* Custom events to trigger application deploys
* Automatically add nodes to Nagios to be monitored
* Propagating changes to configuration
* Update DNS reconds to reflect cluster changes
* As a building block for service discovery

# Serf vs ALL

## ZooKeeper, doozerd, etcd

ZooKeeper, doozerd and etcd are all similar in their client/server
architecture. All three have server nodes that require a quorum of
nodes to operate (usually a simple majority). They are strongly consistent,
and expose various primitives that can be used through client libraries within
applications to build complex distributed systems.

Serf has a radically different architecture based on gossip and provides a
smaller feature set. Serf only provides membership, failure detection
and user events. Serf is designed to operate under network partitions,
and is based on eventual consistency. Designed as a tool, it is friendly
for both system administrators and application developers.

ZooKeeper et al. by contrast are much more complex, and cannot be used directly
as a tool. Application developers must use libraries to build the features
they need, although some libraries exist for common patterns. Most failure
detection schemes built on these systems also have intrinsic scalability issues.
Heartbeating relies on nodes periodically updating a key with a TTL mechanism,
requiring N updates/TTL to be processed by a fixed number of nodes. Alternatively,
ZooKeeper ephemeral nodes require many active connections to be maintained to a few nodes.
The strong consistency provided by these systems is essential for building leader
election or other types of coordination for distributed systems, but it limits
their ability to operate under network partitions. At a minimum, if a majority of
nodes are not available, writes are disallowed. Since a failure is indistinguishable
from a slow response, the performance of these systems may rapidly degrade
under certain network conditions. All of these issues can be highly
problematic when partition tolerance is needed, for example in a service
discovery layer.

Additionally, Serf is not mutually exclusive with any of these strongly
consistent systems. Instead they can be used in combination to create systems
that are more scalable and fault tolerant, without sacraficing features.

## Chef, Puppet, etc.

It may seem strange to compare Serf to configuration management tools,
but most of them provide mechanisms to incorporate global state into the
configuration of a node. For example, Puppet provides exported resouces
and Chef has node searching. As an example, if you generate a config file
for a load balancer to include the web servers, the config management
tool is being used to manage membership.

However, none of these config management tools are designed to perform
this task. They are not designed to propagate information quickly,
handle failure detection or tolerate network partitions. Generally,
they rely on very infrequent convergence runs to bring things up to date.
Lastly, these tools are not friendly for immutable infrastructure as they
require constant operation to keep nodes up to date.

That said, Serf is designed to be used along side config management tools.
Once configured, Serf can be used to handle changes to the cluster and
update configuration files nearly instantly instead of relying on convergence
runs. This way a web server can join a cluster in seconds instead of hours.
The seperation of configuration management and cluster management also has
a number of advantageous side affects. Chef recipes and Puppet manifests become
simpler without global state, periodic runs are no longer required, and
the infrastructure can become immutable.

## Fabric

Fabric is a widely used tool for system administration over SSH. Broadly,
it is used to SSH into a group of nodes and execute commands. Both Fabric
and Serf can be used for service management in different ways. While Fabric
sends commands from a single box, Serf instead rapidly broadcasts a message
to the entire cluster in a distributed fashion. Fabric has a number of advantages
in that it can collect the output of commands and stop execution if an
error is encountered. Serf is unable to do these things since it has no single
destination to send logs to, nor does it have any control flow. However,
Fabric must be provided with a list of nodes to contact, where membership
is built directly into Serf. Serf additional is able to propagate a message
within seconds to an entire cluster, allowing for much higher parallelism.

Fabric is much more capable than Serf at system administration, but it is
limited by its execution speed and lack of node discovery. Combined together,
Fabric can query Serf for nodes and make use of message broadcasts where
appropriate, using direct SSH exection where output is needed.

## Roll your own

Many organizations find themselves building home grown solutions
for service discovery and administration. It is an undisputed fact that
distributed systems are hard; building one is error prone and time consuming.
Most systems cut corners by introducing single points of failure such
as a single Redis or RDBMS. These solutions may work in the short term,
but usually they are not fault tolerant or scalable. Besides these limitations,
they require time and resources to build and maintain.

Serf may not provide the exact feature set needed by an organization,
but it can be used as building block. It provides generally useful features
that are can be used for building distributed systems, and requires no
effort to use out of the box.

## What about the CAP theorem?

The CAP theorem states that it is impossible for a service to be consistent,
available and partition tolerant. Serf makes no promise to provide all three,
and strictly provides AP. All updates take time to propagate through a cluster,
and thus updates are only eventually consistent. However, under favorable network
conditions, Serf is consistent within few seconds. In the case of a network partition,
Serf can continue operating normally and when a partition heals, Serf will automatically
reconnect the partitions.

