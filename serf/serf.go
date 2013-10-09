package serf

import (
	"errors"
	"fmt"
	"github.com/hashicorp/memberlist"
	"log"
	"math/rand"
	"net"
	"os"
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
	clock         LamportClock
	config        *Config
	failedMembers []*memberState
	leftMembers   []*memberState
	memberlist    *memberlist.Memberlist
	memberLock    sync.RWMutex
	members       map[string]*memberState

	// Circular buffers for recent intents, used
	// in case we get the intent before the relevent event
	recentLeave      []nodeIntent
	recentLeaveIndex int
	recentJoin       []nodeIntent
	recentJoinIndex  int

	logger     *log.Logger
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
)

func (s MemberStatus) String() string {
	switch s {
	case StatusNone:
		return "none"
	case StatusAlive:
		return "alive"
	case StatusLeaving:
		return "leaving"
	case StatusLeft:
		return "left"
	case StatusFailed:
		return "failed"
	default:
		panic(fmt.Sprintf("unknown MemberStatus: %d", s))
	}
}

// memberState is used to track members that are no longer active due to
// leaving, failing, partitioning, etc. It tracks the member along with
// when that member was marked as leaving.
type memberState struct {
	Member
	joinLTime LamportTime // lamport clock time of join
	leaveTime time.Time   // wall clock time of leave
}

// nodeIntent is used to buffer intents for out-of-order deliveries
type nodeIntent struct {
	LTime LamportTime
	Node  string
}

// Create creates a new Serf instance, starting all the background tasks
// to maintain cluster membership information.
//
// After calling this function, the configuration should no longer be used
// or modified by the caller.
func Create(conf *Config) (*Serf, error) {
	if conf.NodeName == "" {
		// Default the node name to the hostname
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("Error setting NodeName to hostname: %s", err)
		}

		conf.NodeName = hostname
	}

	if conf.BroadcastTimeout == 0 {
		// Set a cautious default for the timeout for leave broadcasts.
		conf.BroadcastTimeout = 5 * time.Second
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

	if conf.QueueDepthWarning == 0 {
		// Set reasonable default for QueueDepthWarning
		conf.QueueDepthWarning = 128
	}

	if conf.RecentIntentBuffer == 0 {
		// Set a reasonable default for RecentJoinBuffer
		conf.RecentIntentBuffer = 128
	}

	if conf.MemberlistConfig == nil {
		conf.MemberlistConfig = memberlist.DefaultConfig()
	}

	if conf.LogOutput == nil {
		conf.LogOutput = os.Stderr
	}

	serf := &Serf{
		config:     conf,
		logger:     log.New(conf.LogOutput, "", log.LstdFlags),
		members:    make(map[string]*memberState),
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

	// Create the buffer for recent intents
	serf.recentJoin = make([]nodeIntent, conf.RecentIntentBuffer)
	serf.recentLeave = make([]nodeIntent, conf.RecentIntentBuffer)

	// Modify the memberlist configuration with keys that we set
	//conf.MemberlistConfig.Events = &memberlist.ChannelEventDelegate{Ch: eventCh}
	conf.MemberlistConfig.Events = &eventDelegate{serf: serf}
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

	num, err := s.memberlist.Join(existing)

	// If we joined any nodes, broadcast the join message
	if num > 0 {
		// Construct message to update our lamport clock
		msg := messageJoin{
			LTime: s.clock.Increment(),
			Node:  s.config.NodeName,
		}

		// Process update locally
		s.handleNodeJoinIntent(&msg)

		// Start broadcasting the update
		if err := s.broadcast(messageJoinType, &msg, nil); err != nil {
			return num, err
		}
	}

	return num, err
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

	// Construct the message for the graceful leave
	msg := messageLeave{
		LTime: s.clock.Increment(),
		Node:  s.config.NodeName,
	}

	// Process the leave locally
	s.handleNodeLeaveIntent(&msg)

	if len(s.members) > 1 {
		notifyCh := make(chan struct{})
		if err := s.broadcast(messageLeaveType, &msg, notifyCh); err != nil {
			return err
		}

		select {
		case <-notifyCh:
		case <-time.After(s.config.BroadcastTimeout):
			return errors.New("timeout while waiting for graceful leave")
		}
	}

	err := s.memberlist.Leave(s.config.BroadcastTimeout)
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
		members = append(members, m.Member)
	}

	return members
}

// RemoveFailedNode forcibly removes a failed node from the cluster
// immediately, instead of waiting for the reaper to eventually reclaim it.
func (s *Serf) RemoveFailedNode(node string) error {
	// Construct the message to broadcast
	msg := messageRemoveFailed{
		LTime: s.clock.Increment(),
		Node:  node,
	}

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
	if err := s.broadcast(messageRemoveFailedType, &msg, notifyCh); err != nil {
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
		s.logger.Println("[WARN] Shutdown without a Leave")
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
func (s *Serf) broadcast(t messageType, msg interface{}, notify chan<- struct{}) error {
	raw, err := encodeMessage(t, msg)
	if err != nil {
		return err
	}

	s.broadcasts.QueueBroadcast(&broadcast{
		msg:    raw,
		notify: notify,
	})

	return nil
}

// handleNodeForceRemove is invoked when we get a messageRemoveFailed
// message.
func (s *Serf) handleNodeForceRemove(remove *messageRemoveFailed) bool {
	// Witness a potentially newer time
	s.clock.Witness(remove.LTime)

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

	// If the message is old, then it is irrelevant and we can skip it
	if remove.LTime <= member.joinLTime {
		return false
	}

	// Update the status to left
	member.Status = StatusLeft

	// Remove from the failed list and add to the left list. We add
	// to the left list so that when we do a sync, other nodes will
	// remove it from their failed list.
	s.failedMembers = removeOldMember(s.failedMembers, member.Name)
	s.leftMembers = append(s.leftMembers, member)
	return true
}

// handleNodeJoin is called when a node join event is received
// from memberlist.
func (s *Serf) handleNodeJoin(n *memberlist.Node) {
	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	var oldStatus MemberStatus
	member, ok := s.members[n.Name]
	if !ok {
		oldStatus = StatusNone
		member = &memberState{
			Member: Member{
				Name:   n.Name,
				Addr:   net.IP(n.Addr),
				Role:   string(n.Meta),
				Status: StatusAlive,
			},
		}

		// Check if we have a join intent and use the LTime
		if join := recentIntent(s.recentJoin, n.Name); join != nil {
			member.joinLTime = join.LTime
		}

		// Check if we have a leave intent
		if leave := recentIntent(s.recentLeave, n.Name); leave != nil {
			if leave.LTime > member.joinLTime {
				member.Status = StatusLeaving
			}
		}

		s.members[n.Name] = member
	} else {
		oldStatus = member.Status
		member.Status = StatusAlive
		member.leaveTime = time.Time{}
	}

	// If node was previously in a failed state, then clean up some
	// internal accounting.
	if oldStatus == StatusFailed {
		s.failedMembers = removeOldMember(s.failedMembers, member.Name)
		s.leftMembers = removeOldMember(s.leftMembers, member.Name)
	}

	// Send an event along
	s.logger.Printf("[INFO] serf: EventMemberJoin: %s %s",
		member.Member.Name, member.Member.Addr)
	if s.config.EventCh != nil {
		s.config.EventCh <- Event{
			Type:    EventMemberJoin,
			Members: []Member{member.Member},
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
		member.leaveTime = time.Now()
		s.leftMembers = append(s.leftMembers, member)
	case StatusAlive:
		member.Status = StatusFailed
		member.leaveTime = time.Now()
		s.failedMembers = append(s.failedMembers, member)
	default:
		// Unknown state that it was in? Just don't do anything
		s.logger.Printf("[WARN] Bad state when leave: %d", member.Status)
		return
	}

	// Send an event along
	event := EventMemberLeave
	eventStr := "EventMemberLeave"
	if member.Status != StatusLeft {
		event = EventMemberFailed
		eventStr = "EventMemberFailed"
	}

	s.logger.Printf("[INFO] serf: %s: %s %s",
		eventStr, member.Member.Name, member.Member.Addr)
	if s.config.EventCh != nil {
		s.config.EventCh <- Event{
			Type:    event,
			Members: []Member{member.Member},
		}
	}
}

// handleNodeLeaveIntent is called when an intent to leave is received.
func (s *Serf) handleNodeLeaveIntent(leaveMsg *messageLeave) bool {
	// Witness a potentially newer time
	s.clock.Witness(leaveMsg.LTime)

	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	member, ok := s.members[leaveMsg.Node]
	if !ok {
		// If we've already seen this message don't rebroadcast
		if recentIntent(s.recentLeave, leaveMsg.Node) != nil {
			return false
		}

		// We don't know this member so store it in a buffer for now
		s.recentLeave[s.recentLeaveIndex] = nodeIntent{
			LTime: leaveMsg.LTime,
			Node:  leaveMsg.Node,
		}
		s.recentLeaveIndex = (s.recentLeaveIndex + 1) % len(s.recentLeave)
		return true
	}

	// If the node isn't alive, then this message is irrelevent and
	// we skip it.
	if member.Status != StatusAlive {
		return false
	}

	// If the message is old, then it is irrelevant and we can skip it
	if leaveMsg.LTime <= member.joinLTime {
		return false
	}

	// Update the status
	member.Status = StatusLeaving
	return true
}

// handleNodeJoinIntent is called when a node broadcasts a
// join message to set the lamport time of its join
func (s *Serf) handleNodeJoinIntent(joinMsg *messageJoin) bool {
	// Witness a potentially newer time
	s.clock.Witness(joinMsg.LTime)

	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	member, ok := s.members[joinMsg.Node]
	if !ok {
		// If we've already seen this message don't rebroadcast
		if recentIntent(s.recentJoin, joinMsg.Node) != nil {
			return false
		}

		// We don't know this member so store it in a buffer for now
		s.recentJoin[s.recentJoinIndex] = nodeIntent{LTime: joinMsg.LTime, Node: joinMsg.Node}
		s.recentJoinIndex = (s.recentJoinIndex + 1) % len(s.recentJoin)
		return true
	}

	// Check if this time is newer than what we have
	if joinMsg.LTime <= member.joinLTime {
		return false
	}

	// Update the LTime
	member.joinLTime = joinMsg.LTime

	// If we are in the leaving state, we should go back to alive,
	// since the leaving message must have been for an older time
	if member.Status == StatusLeaving {
		member.Status = StatusAlive
	}
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
func (s *Serf) reap(old []*memberState, timeout time.Duration) []*memberState {
	now := time.Now()
	n := len(old)
	for i := 0; i < n; i++ {
		m := old[i]

		// Skip if the timeout is not yet reached
		if now.Sub(m.leaveTime) <= timeout {
			continue
		}

		// Delete from the list
		old[i], old[n-1] = old[n-1], nil
		old = old[:n-1]
		n--
		i--

		// Delete from members
		delete(s.members, m.Name)
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

	// Probability we should attempt to reconect is given
	// by num failed / (num members - num failed - num left)
	// This means that we probabilistically expect the cluster
	// to attempt to connect to each failed member once per
	// reconnect interval
	numFailed := float32(len(s.failedMembers))
	numAlive := float32(len(s.members) - len(s.failedMembers) - len(s.leftMembers))
	if numAlive == 0 {
		numAlive = 1 // guard against zero divide
	}
	prob := numFailed / numAlive
	if rand.Float32() > prob {
		s.memberLock.RUnlock()
		return
	}

	// Select a random member to try and join
	idx := int(rand.Uint32() % uint32(n))
	mem := s.failedMembers[idx]
	s.memberLock.RUnlock()

	// Format the addr
	addr := mem.Addr.String()

	// Attempt to join at the memberlist level
	s.memberlist.Join([]string{addr})
}

// removeOldMember is used to remove an old member from a list of old
// members.
func removeOldMember(old []*memberState, name string) []*memberState {
	for i, m := range old {
		if m.Name == name {
			n := len(old)
			old[i], old[n-1] = old[n-1], nil
			return old[:n-1]
		}
	}

	return old
}

// recentIntent checks the recent intent buffer for a matching
// entry for a given node, and either returns the message or nil
func recentIntent(recent []nodeIntent, node string) (intent *nodeIntent) {
	for i := 0; i < len(recent); i++ {
		// Break fast if we hit a zero entry
		if recent[i].LTime == 0 {
			break
		}

		// Check for a node match
		if recent[i].Node == node {
			// Take the most recent entry
			if intent == nil || recent[i].LTime > intent.LTime {
				intent = &recent[i]
			}
		}
	}
	return
}
