package serf2

import (
	"errors"
	"fmt"
	"github.com/hashicorp/memberlist"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
}

// Serf is a single node that is part of a single cluster that gets
// events about joins/leaves/failures/etc. It is created with the Create
// method.
//
// All functions on the Serf structure are safe to call concurrently.
type Serf struct {
	broadcasts    *memberlist.TransmitLimitedQueue
	config        *Config
	failedMembers []*oldMember
	leftMembers   []*oldMember
	memberlist    *memberlist.Memberlist
	memberLock    sync.RWMutex
	members       map[string]*Member

	stateLock  sync.Mutex
	state      SerfState
	shutdownCh chan struct{}
}

// SerfState is the state of the Serf instance.
type SerfState int

const (
	SerfAlive SerfState = iota
	SerfLeft
	SerfShutdown
)

// Member is a single member of the Serf cluster.
type Member struct {
	Name   string
	Addr   net.IP
	Role   string
	Status MemberStatus
}

// MemberStatus is the state that a member is in.
type MemberStatus int

const (
	StatusNone MemberStatus = iota
	StatusAlive
	StatusLeaving
	StatusLeft
	StatusFailed
	StatusPartition
)

// oldMember is used to track members that are no longer active due to
// leaving, failing, partitioning, etc. It tracks the member along with
// when that member was marked as leaving.
type oldMember struct {
	member *Member
	time   time.Time
}

// Create creates a new Serf instance, starting all the background tasks
// to maintain cluster membership information.
//
// After calling this function, the configuration should no longer be used
// or modified by the caller.
func Create(conf *Config) (*Serf, error) {
	// Some configuration validation first.
	if conf.NodeName == "" {
		return nil, errors.New("NodeName must be set")
	}

	if conf.BroadcastTimeout == 0 {
		// Set a cautious default for the timeout for leave broadcasts.
		conf.BroadcastTimeout = 5 * time.Second
	}

	if conf.LeaveTimeout == 0 {
		// Set a cautious default for a leave timeout.
		conf.LeaveTimeout = 120 * time.Second
	}

	if conf.ReapInterval == 0 {
		// Set a reasonable default for ReapInterval
		conf.ReapInterval = 15 * time.Second
	}

	if conf.ReconnectInterval == 0 {
		// Set a reasonable default for ReconnectInterval
		conf.ReconnectInterval = 30 * time.Second
	}

	if conf.ReconnectTimeout == 0 {
		// Set a reasonable default for ReconnectTimeout
		conf.ReconnectTimeout = 24 * time.Hour
	}

	if conf.TombstoneTimeout == 0 {
		// Set a reasonable default for TombstoneTimeout
		conf.TombstoneTimeout = 24 * time.Hour
	}

	serf := &Serf{
		config:     conf,
		members:    make(map[string]*Member),
		shutdownCh: make(chan struct{}),
		state:      SerfAlive,
	}

	if conf.CoalescePeriod > 0 && conf.EventCh != nil {
		// Event coalescence is enabled, setup the channel.
		conf.EventCh = coalescedEventCh(conf.EventCh, serf.shutdownCh,
			conf.CoalescePeriod, conf.QuiescentPeriod)
	}

	// Setup the broadcast queue, which we use to send our own custom
	// broadcasts along the gossip channel.
	serf.broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			serf.memberLock.RLock()
			defer serf.memberLock.RUnlock()
			return len(serf.members)
		},
		RetransmitMult: conf.MemberlistConfig.RetransmitMult,
	}

	// Setup the channel used for handling node events. We make this buffered
	// in case our handling can't keep up with the events. The buffer is
	// probably too big, but we'd rather be safe than lose events.
	eventCh := make(chan memberlist.NodeEvent, 64)

	// Modify the memberlist configuration with keys that we set
	conf.MemberlistConfig.Events = &memberlist.ChannelEventDelegate{Ch: eventCh}
	conf.MemberlistConfig.Delegate = &delegate{serf: serf}
	conf.MemberlistConfig.Name = conf.NodeName

	// Create the underlying memberlist that will manage membership
	// and failure detection for the Serf instance.
	memberlist, err := memberlist.Create(conf.MemberlistConfig)
	if err != nil {
		return nil, err
	}

	serf.memberlist = memberlist

	// Start the background tasks. See the documentation above each method
	// for more information on their role.
	go serf.handleNodeEvents(eventCh)
	go serf.handleReap()
	go serf.handleReconnect()

	return serf, nil
}

// Join joins an existing Serf cluster. Returns the number of nodes
// successfully contacted. The returned error will be non-nil only in the
// case that no nodes could be contacted.
func (s *Serf) Join(existing []string) (int, error) {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()

	if s.state == SerfShutdown {
		panic("Serf can't Join after Shutdown")
	}

	return s.memberlist.Join(existing)
}

// Leave gracefully exits the cluster. It is safe to call this multiple
// times.
func (s *Serf) Leave() error {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()

	if s.state == SerfLeft {
		return nil
	} else if s.state == SerfShutdown {
		panic("Leave after Shutdown")
	}

	s.memberLock.RLock()
	defer s.memberLock.RUnlock()

	// If we have more than one member (more than ourself), then we need
	// to broadcast that we intend to gracefully leave.
	if len(s.members) > 1 {
		msg := messageLeave{Node: s.config.NodeName}
		notifyCh := make(chan struct{})
		if err := s.broadcast(messageLeaveType, msg.Node, &msg, notifyCh); err != nil {
			return err
		}

		select {
		case <-notifyCh:
		case <-time.After(s.config.BroadcastTimeout):
			return errors.New("timeout while waiting for graceful leave")
		}
	}

	err := s.memberlist.Leave()
	if err != nil {
		return err
	}

	s.state = SerfLeft
	return nil
}

// Members returns a point-in-time snapshot of the members of this cluster.
func (s *Serf) Members() []Member {
	s.memberLock.RLock()
	defer s.memberLock.RUnlock()

	members := make([]Member, 0, len(s.members))
	for _, m := range s.members {
		members = append(members, *m)
	}

	return members
}

// RemoveFailedNode forcibly removes a failed node from the cluster
// immediately, instead of waiting for the reaper to eventually reclaim it.
func (s *Serf) RemoveFailedNode(node string) error {
	// Construct the message to broadcast
	msg := messageRemoveFailed{Node: node}

	// Process our own event
	s.handleNodeForceRemove(&msg)

	// If we have no members, then we don't need to broadcast
	s.memberLock.RLock()
	if len(s.members) <= 1 {
		s.memberLock.RUnlock()
		return nil
	}
	s.memberLock.RUnlock()

	// Broadcast the remove
	notifyCh := make(chan struct{})
	if err := s.broadcast(messageRemoveFailedType, msg.Node, &msg, notifyCh); err != nil {
		return err
	}

	// Wait for the broadcast
	select {
	case <-notifyCh:
	case <-time.After(s.config.BroadcastTimeout):
		return fmt.Errorf("timed out broadcasting node removal")
	}

	return nil
}

// Shutdown forcefully shuts down the Serf instance, stopping all network
// activity and background maintenance associated with the instance.
//
// This is not a graceful shutdown, and should be preceeded by a call
// to Leave. Otherwise, other nodes in the cluster will detect this node's
// exit as a node failure.
//
// It is safe to call this method multiple times.
func (s *Serf) Shutdown() error {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()

	if s.state == SerfShutdown {
		return nil
	}

	if s.state != SerfLeft {
		log.Println("[WARN] Shutdown without a Leave")
	}

	err := s.memberlist.Shutdown()
	if err != nil {
		return err
	}

	s.state = SerfShutdown
	close(s.shutdownCh)
	return nil
}

// State is the current state of this Serf instance.
func (s *Serf) State() SerfState {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()
	return s.state
}

// broadcast takes a Serf message type, encodes it for the wire, and queues
// the broadcast. If a notify channel is given, this channel will be closed
// when the broadcast is sent.
func (s *Serf) broadcast(t messageType, key string, msg interface{}, notify chan<- struct{}) error {
	raw, err := encodeMessage(t, msg)
	if err != nil {
		return err
	}

	s.broadcasts.QueueBroadcast(&broadcast{
		key:    key,
		msg:    raw,
		notify: notify,
	})

	return nil
}

// handleNodeForceRemove is invoked when we get a messageRemoveFailed
// message.
func (s *Serf) handleNodeForceRemove(remove *messageRemoveFailed) bool {
	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	member, ok := s.members[remove.Node]
	if !ok {
		return false
	}

	// If the node isn't failed, then do nothing
	if member.Status != StatusFailed {
		return false
	}

	// Update the status to left
	member.Status = StatusLeft

	// Remove from the failed list and add to the left list. We add
	// to the left list so that when we do a sync, other nodes will
	// remove it from their failed list.
	s.failedMembers = removeOldMember(s.failedMembers, member.Name)
	s.leftMembers = append(s.leftMembers, &oldMember{
		member: member,
		time:   time.Now(),
	})

	return true
}

// handleNodeEvents sits in a loop until shutdown, reading events from
// the given channel and processing them in order to update state within
// the Serf.
func (s *Serf) handleNodeEvents(ch <-chan memberlist.NodeEvent) {
	// TODO(mitchellh): handle shutdown
	for {
		event := <-ch
		switch event.Event {
		case memberlist.NodeJoin:
			s.handleNodeJoin(event.Node)
		case memberlist.NodeLeave:
			s.handleNodeLeave(event.Node)
		default:
			panic(fmt.Sprintf("unknown event type: %#v", event))
		}
	}
}

// handleNodeJoin is called when a node join event is received
// from memberlist.
func (s *Serf) handleNodeJoin(n *memberlist.Node) {
	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	oldStatus := StatusNone
	member, ok := s.members[n.Name]
	if !ok {
		member = &Member{
			Name: n.Name,
			Addr: net.IP(n.Addr),
			Role: string(n.Meta),
		}

		s.members[n.Name] = member
	} else {
		oldStatus = member.Status
	}

	member.Status = StatusAlive

	// If our state didn't somehow change, then ignore it.
	if oldStatus == member.Status {
		return
	}

	// If node was previously in a failed state, then clean up some
	// internal accounting.
	if oldStatus == StatusFailed {
		s.failedMembers = removeOldMember(s.failedMembers, member.Name)
		s.leftMembers = removeOldMember(s.leftMembers, member.Name)
	}

	// Send an event along
	if s.config.EventCh != nil {
		s.config.EventCh <- Event{
			Type:    EventMemberJoin,
			Members: []Member{*member},
		}
	}
}

// handleNodeLeave is called when a node leave event is received
// from memberlist.
func (s *Serf) handleNodeLeave(n *memberlist.Node) {
	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	member, ok := s.members[n.Name]
	if !ok {
		// We've never even heard of this node that is supposedly
		// leaving. Just ignore it completely.
		return
	}

	switch member.Status {
	case StatusLeaving:
		member.Status = StatusLeft
		s.leftMembers = append(s.leftMembers, &oldMember{
			member: member,
			time:   time.Now(),
		})
	case StatusAlive:
		member.Status = StatusFailed
		s.failedMembers = append(s.failedMembers, &oldMember{
			member: member,
			time:   time.Now(),
		})
	default:
		// Unknown state that it was in? Just don't do anything
		log.Printf("[WARN] Bad state when leave: %d", member.Status)
		return
	}

	// Send an event along
	if s.config.EventCh != nil {
		event := EventMemberLeave
		if member.Status != StatusLeft {
			event = EventMemberFailed
		}

		s.config.EventCh <- Event{
			Type:    event,
			Members: []Member{*member},
		}
	}
}

// handleNodeLeaveIntent is called when an intent to leave is received.
func (s *Serf) handleNodeLeaveIntent(leaveMsg *messageLeave) bool {
	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	member, ok := s.members[leaveMsg.Node]
	if !ok {
		// We don't know this member so don't rebroadcast.
		return false
	}

	// If the node isn't alive, then this message is irrelevent and
	// we skip it.
	if member.Status != StatusAlive {
		return false
	}

	member.Status = StatusLeaving

	// Schedule a timer to unmark the leave intention after timeout
	time.AfterFunc(s.config.LeaveTimeout, func() {
		s.resetLeaveIntent(member)
	})

	return true
}

// handleReap periodically reaps the list of failed and left members.
func (s *Serf) handleReap() {
	for {
		select {
		case <-time.After(s.config.ReapInterval):
			s.memberLock.Lock()
			s.failedMembers = s.reap(s.failedMembers, s.config.ReconnectTimeout)
			s.leftMembers = s.reap(s.leftMembers, s.config.TombstoneTimeout)
			s.memberLock.Unlock()
		case <-s.shutdownCh:
			return
		}
	}
}

// handleReconnect attempts to reconnect to recently failed nodes
// on configured intervals.
func (s *Serf) handleReconnect() {
	for {
		select {
		case <-time.After(s.config.ReconnectInterval):
			s.reconnect()
		case <-s.shutdownCh:
			return
		}
	}
}

// reap is called with a list of old members and a timeout, and removes
// members that have exceeded the timeout. The members are removed from
// both the old list and the members itself. Locking is left to the caller.
func (s *Serf) reap(old []*oldMember, timeout time.Duration) []*oldMember {
	now := time.Now()
	n := len(old)
	for i := 0; i < n; i++ {
		m := old[i]

		// Skip if the timeout is not yet reached
		if now.Sub(m.time) <= timeout {
			continue
		}

		// Delete from the list
		old[i], old[n-1] = old[n-1], nil
		old = old[:n-1]
		n--
		i--

		// Delete from members
		delete(s.members, m.member.Name)
	}

	return old
}

// reconnect attempts to reconnect to recently fail nodes.
func (s *Serf) reconnect() {
	s.memberLock.RLock()

	// Nothing to do if there are no failed members
	n := len(s.failedMembers)
	if n == 0 {
		s.memberLock.RUnlock()
		return
	}

	// Select a random member to try and join
	idx := int(rand.Uint32() % uint32(n))
	mem := s.failedMembers[idx]
	s.memberLock.RUnlock()

	// Format the addr
	addr := mem.member.Addr.String()

	// Attempt to join at the memberlist level
	s.memberlist.Join([]string{addr})
}

func (s *Serf) resetLeaveIntent(m *Member) {
	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	if m.Status == StatusLeaving {
		m.Status = StatusAlive
	}
}

// removeOldMember is used to remove an old member from a list of old
// members.
func removeOldMember(old []*oldMember, name string) []*oldMember {
	for i, m := range old {
		if m.member.Name == name {
			n := len(old)
			old[i], old[n-1] = old[n-1], nil
			return old[:n-1]
		}
	}

	return old
}
