package serf

import (
	"github.com/hashicorp/memberlist"
	"time"
)

type statusChange struct {
	member    *Member
	oldStatus MemberStatus
	newStatus MemberStatus
}

// changeHandler is a long running routine to coalesce updates,
// and apply a partition detection heuristic
func (s *Serf) changeHandler() {
	// Run until indicated otherwise
	for s.coalesceUpdates() {
	}
}

// coalesceUpdates will collect all the changes we receive until we
// either reach a quiescent period, or we reach the maximum coaslecing time.
// The coalesced updates are then forwarded to the Delegate
func (s *Serf) coalesceUpdates() bool {
	initialStatus := make(map[*Member]MemberStatus)
	endStatus := make(map[*Member]MemberStatus)
	var coalesceDone <-chan time.Time
	var quiescent <-chan time.Time

OUTER:
	for {
		select {
		case c := <-s.changeCh:
			// Mark the initial and end status of the member
			if _, ok := initialStatus[c.member]; !ok {
				initialStatus[c.member] = c.oldStatus
			}
			endStatus[c.member] = c.newStatus

			// Setup an end timer if none exists
			if coalesceDone == nil {
				coalesceDone = time.After(s.conf.MaxCoalesceTime)
			}

			// Setup a new quiescent timer
			quiescent = time.After(s.conf.MinQuiescentTime)

		case <-coalesceDone:
			break OUTER
		case <-quiescent:
			break OUTER
		case <-s.shutdownCh:
			return false
		}
	}

	// Fire any relevant events
	s.invokeDelegate(initialStatus, endStatus)
	return true
}

// partitionedNodes into various groups based on their start and end states
func partitionEvents(initial, end map[*Member]MemberStatus) (joined, left, failed, partitioned []*Member) {
	for member, endState := range end {
		initState := initial[member]

		// If a node is flapping, ignore it
		if endState == initState {
			continue
		}

		switch endState {
		case StatusAlive:
			joined = append(joined, member)
		case StatusLeft:
			left = append(left, member)
		case StatusFailed:
			failed = append(failed, member)
		case StatusPartitioned:
			partitioned = append(partitioned, member)
		}
	}
	return
}

// invokeDelegate is called to invoke the various delegate events
// after the updates have been coalesced
func (s *Serf) invokeDelegate(initial, end map[*Member]MemberStatus) {
	// Bail if no delegate
	d := s.conf.Delegate
	if d == nil {
		return
	}

	// Partition the nodes
	joined, left, failed, partitioned := partitionEvents(initial, end)

	// Invoke appropriate callbacks
	if len(joined) > 0 {
		d.MembersJoined(joined)
	}
	if len(left) > 0 {
		d.MembersLeft(left)
	}
	if len(failed) > 0 {
		d.MembersFailed(failed)
	}
	if len(partitioned) > 0 {
		d.MembersPartitioned(partitioned)
	}
}

// nodeJoin is fired when memberlist detects a node join
func (s *Serf) nodeJoin(n *memberlist.Node) {
	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	// Check if we know about this node already
	mem, ok := s.memberMap[n.Name]
	oldStatus := StatusNone
	if !ok {
		mem = &Member{
			Name:   n.Name,
			Addr:   n.Addr,
			Role:   string(n.Meta),
			Status: StatusAlive,
		}
		s.memberMap[n.Name] = mem
		s.members = append(s.members, mem)
	} else {
		oldStatus = mem.Status
		mem.Status = StatusAlive
	}

	// Notify about change
	s.changeCh <- statusChange{mem, oldStatus, StatusAlive}

	// Check if we should unsuspect
	if oldStatus == StatusFailed || oldStatus == StatusPartitioned {
		s.unsuspectPartition(mem)
	}
}

// nodeLeave is fired when memberlist detects a node join
func (s *Serf) nodeLeave(n *memberlist.Node) {
	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	// Check if we know about this node
	mem, ok := s.memberMap[n.Name]
	if !ok {
		return
	}

	// Determine the state change
	oldStatus := mem.Status
	switch mem.Status {
	case StatusAlive:
		mem.Status = StatusFailed
	case StatusLeaving:
		mem.Status = StatusLeft
	}

	// Check if we should notify about a change
	s.changeCh <- statusChange{mem, oldStatus, mem.Status}

	// Suspect a partition on failure
	if mem.Status == StatusFailed {
		s.suspectPartition(mem)
	}
}

// intendLeave is invoked when we get a message indicating
// an intention to leave. Returns true if we should re-broadcast
func (s *Serf) intendLeave(l *leave) bool {
	s.memberLock.Lock()
	defer s.memberLock.Unlock()

	// Check if we know about this node
	mem, ok := s.memberMap[l.Node]
	if !ok {
		return false // unknown, don't rebroadcast
	}

	// If the node is currently alive, then mark as a pending leave
	// and re-broadcast
	if mem.Status == StatusAlive {
		mem.Status = StatusLeaving
		return true
	}

	// State update not relevant, ignore it
	return false
}
