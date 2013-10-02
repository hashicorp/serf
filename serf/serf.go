package serf

import (
	"github.com/hashicorp/memberlist"
	"sync"
)

const (
	StatusAlive = iota
	StatusLeaving
	StatusLeft
	StatusFailed
	StatusPartitioned // Partitioned is a best guess, should be treated as failed
)

type Serf struct {
	conf       *Config
	memberlist *memberlist.Memberlist
	joinCh     chan *memberlist.Node
	leaveCh    chan *memberlist.Node

	memberLock sync.RWMutex
	members    []*Member
	memberMap  map[string]*Member
}

// Member represents a single member in the gossip pool
type Member struct {
	Name   string
	Addr   []byte
	Role   string
	Status int
}

type EventDelegate interface {
	// MembersJoined invoked when members have joined the cluster
	MembersJoined([]*Member)

	// MembersLeft invoked when members have left the cluster gracefully
	MembersLeft([]*Member)

	// MembersFailed invoked when members are unreachable by the cluster without
	// previously announcing their intention to leave
	MembersFailed([]*Member)

	// MembersPartitioned invoked when members are unreachable by cluster without
	// previously announcing their intention to leave. Partitions are impossible
	// to tell apart from failures, so this is more of a heuristic based on the
	// likelyhood of simultaneous failures. It should not be treated as exact.
	MembersPartitioned([]*Member)
}

// Start is used to initialize a new Serf instance
func Start(conf *Config) (*Serf, error) {
	serf := &Serf{
		conf:    conf,
		joinCh:  make(chan *memberlist.Node, 64),
		leaveCh: make(chan *memberlist.Node, 64),
	}

	// Create the memberlist config
	mlConf := memberlistConfig(conf)
	mlConf.JoinCh = serf.joinCh
	mlConf.LeaveCh = serf.leaveCh
	mlConf.UserDelegate = serf

	// Attempt to create the
	memb, err := memberlist.Create(mlConf)
	if err != nil {
		return nil, err
	}
	serf.memberlist = memb

	// Done
	return serf, nil
}
