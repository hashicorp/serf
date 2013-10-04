package serf

import (
	"fmt"
	"github.com/hashicorp/memberlist"
	"log"
	"sync"
	"time"
)

type MemberStatus int

const (
	StatusNone MemberStatus = iota
	StatusAlive
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
	shutdownCh chan struct{}

	memberLock    sync.RWMutex
	members       map[string]*Member
	failedMembers []*oldMember
	leftMembers   []*oldMember

	detector partitionDetector

	broadcasts *memberlist.TransmitLimitedQueue

	changeCh chan statusChange
}

// Member represents a single member in the gossip pool
type Member struct {
	Name   string
	Addr   []byte
	Role   string
	Status MemberStatus
}

// EventDelegate is the interface that must be implemented by a client that wants to
// receive coalesced event notifications. These methods will never be called concurrently.
// New events will not be coalesced until the delegate returns, so the delegate should minimize
// time spent in the callbacks.
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

// newSerf is used to construct a serf struct from its config
func newSerf(conf *Config) *Serf {
	serf := &Serf{
		conf:       conf,
		joinCh:     make(chan *memberlist.Node, 128),
		leaveCh:    make(chan *memberlist.Node, 128),
		shutdownCh: make(chan struct{}),
		members:    make(map[string]*Member),
		changeCh:   make(chan statusChange, 4096),
	}

	// Select a partition detector
	if conf.PartitionCount > 0 && conf.PartitionInterval > 0 {
		serf.detector = newPartitionRing(conf.PartitionCount, conf.PartitionInterval)
	} else {
		// Parititon detection disabled
		serf.detector = noopDetector{}
	}

	// Create the broadcast queue
	serf.broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes:       func() int { return len(serf.members) },
		RetransmitMult: conf.RetransmitMult,
	}
	return serf
}

// Start is used to initialize a new Serf instance
func Start(conf *Config) (*Serf, error) {
	serf := newSerf(conf)

	// Create the memberlist config
	mlConf := memberlistConfig(conf)
	mlConf.Notify = serf
	mlConf.UserDelegate = serf

	// Attempt to create the
	memb, err := memberlist.Create(mlConf)
	if err != nil {
		return nil, err
	}
	serf.memberlist = memb

	// Start the change handler
	go serf.changeHandler()

	// Start the reconnect handler
	go serf.reconnectHandler()

	// Start the reap handler
	go serf.reapHandler()

	// Done
	return serf, nil
}

// Join is used to attempt to join an existing gossip pool
// Returns an error if none of the existing nodes could be contacted
func (s *Serf) Join(existing []string) error {
	// Ensure we have some input
	if len(existing) == 0 {
		return fmt.Errorf("must specify at least one node to join")
	}

	// Use memberlist to perform the join
	_, err := s.memberlist.Join(existing)
	return err
}

// Members provides a point-in-time snapshot of the members
func (s *Serf) Members() []*Member {
	s.memberLock.RLock()
	defer s.memberLock.RUnlock()

	members := make([]*Member, 0, len(s.members))
	for _, m := range s.members {
		newM := &Member{}
		*newM = *m
		members = append(members, newM)
	}

	return members
}

// Leave allows a node to gracefully leave the cluster. This
// should be followed by a call to Shutdown
func (s *Serf) Leave() error {
	var notifyCh chan struct{}
	var l leave
	var mem *Member
	var ok bool

	// No need to broadcast if there is nobody else
	if len(s.members) <= 1 {
		goto AFTER_BROADCAST
	}

	// Set our own status to leaving
	s.memberLock.RLock()
	mem, ok = s.members[s.conf.Hostname]
	if ok {
		mem.Status = StatusLeaving
	}
	s.memberLock.RUnlock()

	// Create a channel to get notified
	notifyCh = make(chan struct{}, 1)

	// Broadcast leave intention
	l = leave{s.conf.Hostname}
	if err := s.encodeBroadcastNotify(leaveMsg, &l, notifyCh); err != nil {
		return err
	}

	select {
	case <-notifyCh:
	case <-time.After(s.conf.LeaveBroadcastTimeout):
		log.Printf("[WARN] Timed out broadcasting leave intention")
	}

AFTER_BROADCAST:
	// Broadcast our own death
	return s.memberlist.Leave()
}

// Shutdown is used to shutdown all the listeners. It is not graceful,
// and should be preceeded by a call to Leave.
func (s *Serf) Shutdown() error {
	// Emit once per background routine (eventHandler, changeHandler)
	close(s.shutdownCh)
	return s.memberlist.Shutdown()
}
