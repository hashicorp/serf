package serf

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/hashicorp/memberlist"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

// These are the protocol versions that Serf can _understand_. These are
// Serf-level protocol versions that are passed down as the delegate
// version to memberlist below.
const (
	ProtocolVersionMin uint8 = 0
	ProtocolVersionMax       = 1
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
	// The clocks for different purposes. These MUST be the first things
	// in this struct so due to Golang issue #599.
	clock      LamportClock
	eventClock LamportClock

	broadcasts    *memberlist.TransmitLimitedQueue
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

	eventBroadcasts *memberlist.TransmitLimitedQueue
	eventBuffer     []*userEvents
	eventJoinIgnore bool
	eventMinTime    LamportTime
	eventLock       sync.RWMutex

	logger     *log.Logger
	stateLock  sync.Mutex
	state      SerfState
	shutdownCh chan struct{}
}

// SerfState is the state of the Serf instance.
type SerfState int

const (
	SerfAlive SerfState = iota
	SerfLeaving
	SerfLeft
	SerfShutdown
)

// Member is a single member of the Serf cluster.
type Member struct {
	Name   string
	Addr   net.IP
	Role   string
	Status MemberStatus

	// The minimum, maximum, and current values of the protocol versions
	// and delegate (Serf) protocol versions that each member can understand
	// or is speaking.
	ProtocolMin uint8
	ProtocolMax uint8
	ProtocolCur uint8
	DelegateMin uint8
	DelegateMax uint8
	DelegateCur uint8
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
	statusLTime LamportTime // lamport clock time of last received message
	leaveTime   time.Time   // wall clock time of leave
}

// nodeIntent is used to buffer intents for out-of-order deliveries
type nodeIntent struct {
	LTime LamportTime
	Node  string
}

// userEvent is used to buffer events to prevent re-delivery
type userEvent struct {
	Name    string
	Payload []byte
}

func (ue *userEvent) Equals(other *userEvent) bool {
	if ue.Name != other.Name {
		return false
	}
	if bytes.Compare(ue.Payload, other.Payload) != 0 {
		return false
	}
	return true
}

// userEvents stores all the user events at a specific time
type userEvents struct {
	LTime  LamportTime
	Events []userEvent
}

const (
	UserEventSizeLimit = 128 // Maximum byte size for event name and payload
)

// Create creates a new Serf instance, starting all the background tasks
// to maintain cluster membership information.
//
// After calling this function, the configuration should no longer be used
// or modified by the caller.
func Create(conf *Config) (*Serf, error) {
	if conf.ProtocolVersion < ProtocolVersionMin {
		return nil, fmt.Errorf("Protocol version '%d' too low. Must be in range: [%d, %d]",
			conf.ProtocolVersion, ProtocolVersionMin, ProtocolVersionMax)
	} else if conf.ProtocolVersion > ProtocolVersionMax {
		return nil, fmt.Errorf("Protocol version '%d' too high. Must be in range: [%d, %d]",
			conf.ProtocolVersion, ProtocolVersionMin, ProtocolVersionMax)
	}

	serf := &Serf{
		config:     conf,
		logger:     log.New(conf.LogOutput, "", log.LstdFlags),
		members:    make(map[string]*memberState),
		shutdownCh: make(chan struct{}),
		state:      SerfAlive,
	}

	// Check if serf member event coalescing is enabled
	if conf.CoalescePeriod > 0 && conf.QuiescentPeriod > 0 && conf.EventCh != nil {
		c := &memberEventCoalescer{
			lastEvents:   make(map[string]EventType),
			latestEvents: make(map[string]coalesceEvent),
		}

		conf.EventCh = coalescedEventCh(conf.EventCh, serf.shutdownCh,
			conf.CoalescePeriod, conf.QuiescentPeriod, c)
	}

	// Check if user event coalescing is enabled
	if conf.UserCoalescePeriod > 0 && conf.UserQuiescentPeriod > 0 && conf.EventCh != nil {
		c := &userEventCoalescer{
			events: make(map[string]*latestUserEvents),
		}

		conf.EventCh = coalescedEventCh(conf.EventCh, serf.shutdownCh,
			conf.UserCoalescePeriod, conf.UserQuiescentPeriod, c)
	}

	// Setup the broadcast queue, which we use to send our own custom
	// broadcasts along the gossip channel.
	serf.broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return len(serf.members)
		},
		RetransmitMult: conf.MemberlistConfig.RetransmitMult,
	}
	serf.eventBroadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return len(serf.members)
		},
		RetransmitMult: conf.MemberlistConfig.RetransmitMult,
	}

	// Create the buffer for recent intents
	serf.recentJoin = make([]nodeIntent, conf.RecentIntentBuffer)
	serf.recentLeave = make([]nodeIntent, conf.RecentIntentBuffer)

	// Create a buffer for events
	serf.eventBuffer = make([]*userEvents, conf.EventBuffer)

	// Ensure our lamport clock is at least 1, so that the default
	// join LTime of 0 does not cause issues
	serf.clock.Increment()
	serf.eventClock.Increment()

	// Modify the memberlist configuration with keys that we set
	conf.MemberlistConfig.Events = &eventDelegate{serf: serf}
	conf.MemberlistConfig.Delegate = &delegate{serf: serf}
	conf.MemberlistConfig.DelegateProtocolVersion = conf.ProtocolVersion
	conf.MemberlistConfig.DelegateProtocolMin = ProtocolVersionMin
	conf.MemberlistConfig.DelegateProtocolMax = ProtocolVersionMax
	conf.MemberlistConfig.Name = conf.NodeName
	conf.MemberlistConfig.ProtocolVersion = ProtocolVersionMap[conf.ProtocolVersion]

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
	go serf.checkQueueDepth(conf.QueueDepthWarning, "Intent",
		serf.broadcasts, serf.shutdownCh)
	go serf.checkQueueDepth(conf.QueueDepthWarning, "Event",
		serf.eventBroadcasts, serf.shutdownCh)

	return serf, nil
}

// ProtocolVersion returns the current protocol version in use by Serf.
// This is the Serf protocol version, not the memberlist protocol version.
func (s *Serf) ProtocolVersion() uint8 {
	return s.config.ProtocolVersion
}

// UserEvent is used to broadcast a custom user event with a given
// name and payload. The events must be fairly small, and if the
// size limit is exceeded and error will be returned. If coalesce is enabled,
// nodes are allowed to coalesce this event. Coalescing is only available
// starting in v0.2
func (s *Serf) UserEvent(name string, payload []byte, coalesce bool) error {
	// Check the size limit
	if len(name)+len(payload) > UserEventSizeLimit {
		return fmt.Errorf("user event payload exceeds limit of %d bytes", UserEventSizeLimit)
	}

	// Create a message
	msg := messageUserEvent{
		LTime:   s.eventClock.Time(),
		Name:    name,
		Payload: payload,
		CC:      coalesce,
	}
	s.eventClock.Increment()

	// Process update locally
	s.handleUserEvent(&msg)

	// Start broadcasting the event
	raw, err := encodeMessage(messageUserEventType, &msg)
	if err != nil {
		return err
	}
	s.eventBroadcasts.QueueBroadcast(&broadcast{
		msg: raw,
	})
	return nil
}

// Join joins an existing Serf cluster. Returns the number of nodes
// successfully contacted. The returned error will be non-nil only in the
// case that no nodes could be contacted. If ignoreOld is true, then any
// user messages sent prior to the join will be ignored.
func (s *Serf) Join(existing []string, ignoreOld bool) (int, error) {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()

	if s.state == SerfShutdown {
		return 0, fmt.Errorf("Serf can't Join after Shutdown")
	}

	// Ignore any events from a potential join. This is safe since we hold
	// the stateLock and nobody else can be doing a Join
	if ignoreOld {
		s.eventJoinIgnore = true
		defer func() {
			s.eventJoinIgnore = false
		}()
	}

	// Have memberlist attempt to join
	num, err := s.memberlist.Join(existing)

	// If we joined any nodes, broadcast the join message
	if num > 0 {
		// Start broadcasting the update
		if err := s.broadcastJoin(s.clock.Time()); err != nil {
			return num, err
		}
	}

	return num, err
}

// broadcastJoin broadcasts a new join intent with a
// given clock value. It is used on either join, or if
// we need to refute an older leave intent. Cannot be called
// with the memberLock held.
func (s *Serf) broadcastJoin(ltime LamportTime) error {
	// Construct message to update our lamport clock
	msg := messageJoin{
		LTime: ltime,
		Node:  s.config.NodeName,
	}
	s.clock.Witness(ltime)

	// Process update locally
	s.handleNodeJoinIntent(&msg)

	// Start broadcasting the update
	if err := s.broadcast(messageJoinType, &msg, nil); err != nil {
		s.logger.Printf("[WARN] Failed to broadcast join intent: %v", err)
		return err
	}
	return nil
}

// Leave gracefully exits the cluster. It is safe to call this multiple
// times.
func (s *Serf) Leave() error {
	s.stateLock.Lock()
	defer s.stateLock.Unlock()

	if s.state == SerfLeft {
		return nil
	} else if s.state == SerfShutdown {
		return fmt.Errorf("Leave called after Shutdown")
	}

	// Moving into a temporary Leaving state, rollback on failure
	s.state = SerfLeaving
	defer func() {
		if s.state != SerfLeft {
			s.state = SerfAlive
		}
	}()

	// Construct the message for the graceful leave
	msg := messageLeave{
		LTime: s.clock.Time(),
		Node:  s.config.NodeName,
	}
	s.clock.Increment()

	// Process the leave locally
	s.handleNodeLeaveIntent(&msg)

	// Only broadcast the leave message if there is at least one
	// other node alive.
	if s.hasAliveMembers() {
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

// hasAliveMembers is called to check for any alive members other than
// ourself.
func (s *Serf) hasAliveMembers() bool {
	s.memberLock.RLock()
	defer s.memberLock.RUnlock()

	hasAlive := false
	for _, m := range s.members {
		// Skip ourself, we want to know if OTHER members are alive
		if m.Name == s.config.NodeName {
			continue
		}

		if m.Status == StatusAlive {
			hasAlive = true
			break
		}
	}
	return hasAlive
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
// This also has the effect that Serf will no longer attempt to reconnect
// to this node.
func (s *Serf) RemoveFailedNode(node string) error {
	// Construct the message to broadcast
	msg := messageLeave{
		LTime: s.clock.Time(),
		Node:  node,
	}
	s.clock.Increment()

	// Process our own event
	s.handleNodeLeaveIntent(&msg)

	// If we have no members, then we don't need to broadcast
	if !s.hasAliveMembers() {
		return nil
	}

	// Broadcast the remove
	notifyCh := make(chan struct{})
	if err := s.broadcast(messageLeaveType, &msg, notifyCh); err != nil {
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
			member.statusLTime = join.LTime
		}

		// Check if we have a leave intent
		if leave := recentIntent(s.recentLeave, n.Name); leave != nil {
			if leave.LTime > member.statusLTime {
				member.Status = StatusLeaving
				member.statusLTime = leave.LTime
			}
		}

		s.members[n.Name] = member
	} else {
		oldStatus = member.Status
		member.Status = StatusAlive
		member.leaveTime = time.Time{}
		member.Addr = net.IP(n.Addr)
		member.Role = string(n.Meta)
	}

	// Update the protocol versions every time we get an event
	member.ProtocolMin = n.PMin
	member.ProtocolMax = n.PMax
	member.ProtocolCur = n.PCur
	member.DelegateMin = n.DMin
	member.DelegateMax = n.DMax
	member.DelegateCur = n.DCur

	// If node was previously in a failed state, then clean up some
	// internal accounting.
	// TODO(mitchellh): needs tests to verify not reaped
	if oldStatus == StatusFailed || oldStatus == StatusLeft {
		s.failedMembers = removeOldMember(s.failedMembers, member.Name)
		s.leftMembers = removeOldMember(s.leftMembers, member.Name)
	}

	// Send an event along
	s.logger.Printf("[INFO] serf: EventMemberJoin: %s %s",
		member.Member.Name, member.Member.Addr)
	if s.config.EventCh != nil {
		s.config.EventCh <- MemberEvent{
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
		s.config.EventCh <- MemberEvent{
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

	// If the message is old, then it is irrelevant and we can skip it
	if leaveMsg.LTime <= member.statusLTime {
		return false
	}

	// Refute us leaving if we are in the alive state
	// Must be done in another goroutine since we have the memberLock
	if leaveMsg.Node == s.config.NodeName && s.state == SerfAlive {
		s.logger.Printf("[DEBUG] Refuting an older leave intent")
		go s.broadcastJoin(s.clock.Time())
		return false
	}

	// State transition depends on current state
	switch member.Status {
	case StatusAlive:
		member.Status = StatusLeaving
		member.statusLTime = leaveMsg.LTime
		return true
	case StatusFailed:
		member.Status = StatusLeft
		member.statusLTime = leaveMsg.LTime

		// Remove from the failed list and add to the left list. We add
		// to the left list so that when we do a sync, other nodes will
		// remove it from their failed list.
		s.failedMembers = removeOldMember(s.failedMembers, member.Name)
		s.leftMembers = append(s.leftMembers, member)

		return true
	default:
		return false
	}
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
	if joinMsg.LTime <= member.statusLTime {
		return false
	}

	// Update the LTime
	member.statusLTime = joinMsg.LTime

	// If we are in the leaving state, we should go back to alive,
	// since the leaving message must have been for an older time
	if member.Status == StatusLeaving {
		member.Status = StatusAlive
	}
	return true
}

// handleUserEvent is called when a user event broadcast is
// received. Returns if the message should be rebroadcast.
func (s *Serf) handleUserEvent(eventMsg *messageUserEvent) bool {
	// Witness a potentially newer time
	s.eventClock.Witness(eventMsg.LTime)

	s.eventLock.Lock()
	defer s.eventLock.Unlock()

	// Ignore if it is before our minimum event time
	if eventMsg.LTime < s.eventMinTime {
		return false
	}

	// Check if this message is too old
	curTime := s.eventClock.Time()
	if curTime > LamportTime(len(s.eventBuffer)) &&
		eventMsg.LTime < curTime-LamportTime(len(s.eventBuffer)) {
		s.logger.Printf(
			"[WARN] serf: received old event %s from time %d (current: %d)",
			eventMsg.Name,
			eventMsg.LTime,
			s.eventClock.Time())
		return false
	}

	// Check if we've already seen this
	idx := eventMsg.LTime % LamportTime(len(s.eventBuffer))
	seen := s.eventBuffer[idx]
	userEvent := userEvent{Name: eventMsg.Name, Payload: eventMsg.Payload}
	if seen != nil && seen.LTime == eventMsg.LTime {
		for _, previous := range seen.Events {
			if previous.Equals(&userEvent) {
				return false
			}
		}
	} else {
		seen = &userEvents{LTime: eventMsg.LTime}
		s.eventBuffer[idx] = seen
	}

	// Add to recent events
	seen.Events = append(seen.Events, userEvent)

	if s.config.EventCh != nil {
		s.config.EventCh <- UserEvent{
			LTime:    eventMsg.LTime,
			Name:     eventMsg.Name,
			Payload:  eventMsg.Payload,
			Coalesce: eventMsg.CC,
		}
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
		s.logger.Printf("[DEBUG] serf: forgoing reconnect for random throttling")
		return
	}

	// Select a random member to try and join
	idx := int(rand.Uint32() % uint32(n))
	mem := s.failedMembers[idx]
	s.memberLock.RUnlock()
	s.logger.Printf("[INFO] serf: attempting reconnect to %v %v", mem.Name, net.IP(mem.Addr))

	// Format the addr
	addr := mem.Addr.String()

	// Attempt to join at the memberlist level
	s.memberlist.Join([]string{addr})
}

// checkQueueDepth periodically checks the size of a queue to see if
// it is too large
func (s *Serf) checkQueueDepth(limit int, name string, queue *memberlist.TransmitLimitedQueue, shutdownCh chan struct{}) {
	for {
		select {
		case <-time.After(time.Second):
			numq := queue.NumQueued()
			if numq >= limit {
				s.logger.Printf("[WARN] %s queue depth: %d", name, numq)
			}
		case <-shutdownCh:
			return
		}
	}
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
