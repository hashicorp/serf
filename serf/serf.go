package serf

import (
	"fmt"
	"github.com/hashicorp/memberlist"
	"log"
	"sync"
	"time"
)

const (
	StatusNone = iota
	StatusAlive
	StatusLeaving
	StatusLeft
	StatusFailed
	StatusPartitioned // Partitioned is a best guess, should be treated as failed
)

type Agent struct {
	conf       *Config
	memberlist *memberlist.Memberlist
	joinCh     chan *memberlist.Node
	leaveCh    chan *memberlist.Node
	shutdownCh chan struct{}

	memberLock sync.RWMutex
	members    []*Member
	memberMap  map[string]*Member

	broadcasts *memberlist.TransmitLimitedQueue

	changeCh chan statusChange
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
func Start(conf *Config) (*Agent, error) {
	serf := &Agent{
		conf:       conf,
		joinCh:     make(chan *memberlist.Node, 64),
		leaveCh:    make(chan *memberlist.Node, 64),
		shutdownCh: make(chan struct{}, 4),
		changeCh:   make(chan statusChange, 1024),
	}

	// Create the broadcast queue
	serf.broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes:       func() int { return len(serf.members) },
		RetransmitMult: conf.RetransmitMult,
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

	// Start the event handelr
	go serf.eventHandler()

	// Start the change handler
	go serf.changeHandler()

	// Done
	return serf, nil
}

// Join is used to attempt to join an existing gossip pool
// Returns an error if none of the existing nodes could be contacted
func (s *Agent) Join(existing []string) error {
	// Ensure we have some input
	if len(existing) == 0 {
		return fmt.Errorf("must specify at least one node to join")
	}

	// Use memberlist to perform the join
	_, err := s.memberlist.Join(existing)
	return err
}

// Members provides a point-in-time snapshot of the members
func (s *Agent) Members() []*Member {
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
func (s *Agent) Leave() error {
	var notifyCh chan struct{}
	var l leave

	// No need to broadcast if there is nobody else
	if len(s.members) <= 1 {
		goto AFTER_BROADCAST
	}

	// Create a channel to get notified
	notifyCh = make(chan struct{}, 1)

	// Broadcast leave intention
	l = leave{s.conf.Hostname}
	if err := s.encodeBroadcastNotify(leaveMsg, &l, notifyCh); err != nil {
		return err
	}

	select {
	case <-notifyCh:
	case <-time.After(s.conf.LeaveTimeout):
		log.Printf("[WARN] Timed out broadcasting leave intention")
	}

AFTER_BROADCAST:
	// Broadcast our own death
	return s.memberlist.Leave()
}

// Shutdown is used to shutdown all the listeners. It is not graceful,
// and should be preceeded by a call to Leave.
func (s *Agent) Shutdown() error {
	// Emit once per background routine (eventHandler, changeHandler)
	for i := 0; i < 2; i++ {
		s.shutdownCh <- struct{}{}
	}
	return s.memberlist.Shutdown()
}
