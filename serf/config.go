package serf

import (
	"github.com/hashicorp/memberlist"
	"time"
)

type Config struct {
	Hostname string // Node name (FQDN)
	Role     string // Role in the gossip pool

	MaxCoalesceTime  time.Duration // Maximum period of event coalescing for updates
	MinQuiescentTime time.Duration // Minimum period of quiescence for updates. This has lower precedence then MaxCoalesceTime

	LeaveTimeout time.Duration // Timeout for leaving

	PartitionCount    int           // If PartitionCount nodes fail in PartitionInvernal, it is considered a partition
	PartitionInterval time.Duration // ParitionInterval must be < MaxCoalesceTime

	GossipBindAddr   string        // Binding address
	GossipPort       int           // TCP and UDP ports for gossip
	GossipTCPTimeout time.Duration // TCP timeout
	IndirectChecks   int           // Number of indirect checks to use
	RetransmitMult   int           // Retransmits = RetransmitMult * log(N+1)
	SuspicionMult    int           // Suspicion time = SuspcicionMult * log(N+1) * Interval
	PushPullInterval time.Duration // How often we do a Push/Pull update
	RTT              time.Duration // 99% precentile of round-trip-time
	ProbeInterval    time.Duration // Failure probing interval length
	GossipNodes      int           // Number of nodes to gossip to per GossipInterval
	GossipInterval   time.Duration // Gossip interval for non-piggyback messages (only if GossipNodes > 0)

	Delegate EventDelegate // Notified on member events
}

// memberlistConfig constructs the memberlist configuration from our configuration
func memberlistConfig(conf *Config) *memberlist.Config {
	mc := &memberlist.Config{}
	mc.Name = conf.Hostname
	mc.BindAddr = conf.GossipBindAddr
	mc.UDPPort = conf.GossipPort
	mc.TCPPort = conf.GossipPort
	mc.TCPTimeout = conf.GossipTCPTimeout
	mc.IndirectChecks = conf.IndirectChecks
	mc.RetransmitMult = conf.RetransmitMult
	mc.SuspicionMult = conf.SuspicionMult
	mc.PushPullInterval = conf.PushPullInterval
	mc.RTT = conf.RTT
	mc.ProbeInterval = conf.ProbeInterval
	mc.GossipNodes = conf.GossipNodes
	mc.GossipInterval = conf.GossipInterval
	return mc
}
