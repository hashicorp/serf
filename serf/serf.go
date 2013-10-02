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

// Join is used to attempt to join an existing gossip pool
// Returns an error if none of the existing nodes could be contacted
func (s *Serf) Join(existing []string) error {
	return nil
}

// Members provides a point-in-time snapshot of the members
func (s *Serf) Members() []*Member {
	s.memberLock.RLock()
	defer s.memberLock.RUnlock()

	members := make([]*Member, len(s.members))
	for idx, m := range s.members {
		newM := &Member{}
		*newM = *m
		members[idx] = newM
	}

	return members
}

// Leave allows a node to gracefully leave the cluster. This
// should be followed by a call to Shutdown
func (s *Serf) Leave() error {
	return nil
}

// Shutdown is used to shutdown all the listeners. It is not graceful,
// and should be preceeded by a call to Leave.
func (s *Serf) Shutdown() error {
	return nil
}
